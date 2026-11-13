package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type GrantPlayAssetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantPlayAssetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantPlayAssetLogic {
	return &GrantPlayAssetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantPlayAsset 玩法实物发奖：按 grantNo 幂等插入资产行（asset_no="pa_"+grantNo）。
//
// 与 draw 域 GrantAsset 的区别：不反查开奖记录，奖品信息由 play 域快照传入；
// 资产状态为 AVAILABLE，由后续履约/自提流程消费。重复调用返回已有资产。
func (l *GrantPlayAssetLogic) GrantPlayAsset(in *money.GrantPlayAssetReq) (*money.GrantPlayAssetResp, error) {
	if in.GrantNo == "" || in.UserId <= 0 || in.SkuId <= 0 || in.ItemName == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=grantNo/userId/skuId/itemName不能为空")
	}
	assetNo := "pa_" + in.GrantNo

	var existing gacha_money.Assets
	err := l.svcCtx.DB.Where("asset_no = ?", assetNo).First(&existing).Error
	if err == nil {
		return &money.GrantPlayAssetResp{AssetId: existing.Id, ItemName: existing.ItemName, State: int32(existing.State)}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("GrantPlayAsset query existing failed, assetNo: %s, err: %v", assetNo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	now := time.Now()
	asset := &gacha_money.Assets{
		AssetNo:   assetNo,
		UserId:    in.UserId,
		PoolId:    in.PoolId,
		SkuId:     in.SkuId,
		Source:    "play",
		ItemName:  in.ItemName,
		State:     assetStateAvailable,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := l.svcCtx.AssetModel.Insert(l.ctx, nil, asset); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 并发重复：先到者已插入，读回返回
			if err := l.svcCtx.DB.Where("asset_no = ?", assetNo).First(&existing).Error; err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			return &money.GrantPlayAssetResp{AssetId: existing.Id, ItemName: existing.ItemName, State: int32(existing.State)}, nil
		}
		l.Errorf("GrantPlayAsset insert failed, assetNo: %s, err: %v", assetNo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("GrantPlayAsset success, userId: %d, assetNo: %s, skuId: %d", in.UserId, assetNo, in.SkuId)
	return &money.GrantPlayAssetResp{AssetId: asset.Id, ItemName: asset.ItemName, State: int32(asset.State)}, nil
}
