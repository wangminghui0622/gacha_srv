package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GrantAssetRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantAssetRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantAssetRevertLogic {
	return &GrantAssetRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantAssetRevert SAGA 补偿分支 06：资产 DELIVERED→CANCELED
//
// 跳过情形（均如实区分）：
//  1. 屏障级跳过：读库如实返回当前状态；
//  2. 资产不存在 = 空补偿（正向未执行/谢谢参与未落资产）；
//  3. 已 CANCELED = 重复补偿；
//  4. 非 DELIVERED 状态（预留/消耗等后续阶段）= 防御性跳过，不跨阶段撤销。
func (l *GrantAssetRevertLogic) GrantAssetRevert(in *money.GrantAssetReq) (*money.GrantAssetResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=requestId不能为空")
	}
	assetNo := "a_" + in.RequestId

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("GrantAssetRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("GrantAssetRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	statusOut := "canceled"
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)
		// FOR UPDATE 锁资产（不过滤 state：如实区分「已撤销=重复补偿」与「不存在=空补偿」）
		var a gacha_money.Assets
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("asset_no = ?", assetNo).First(&a).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			statusOut = "skipped"
			l.Infof("empty compensate: asset not found, assetNo: %s (forward op never ran or no prize), skip", assetNo)
			return nil
		}
		if err != nil {
			return fmt.Errorf("select asset failed: %w", err)
		}
		if a.State == assetStateCanceled {
			statusOut = "skipped"
			l.Infof("empty compensate: asset already canceled, assetNo: %s, skip", assetNo)
			return nil
		}
		if a.State != assetStateDelivered {
			// 非 DELIVERED（后续阶段状态）：防御性跳过，不跨阶段撤销
			statusOut = "skipped"
			l.Infof("defensive skip: asset state %d not DELIVERED, assetNo: %s", a.State, assetNo)
			return nil
		}
		// DELIVERED→CANCELED 盲更新（行已锁，WHERE state 过滤属防御性编程）
		result := db.Model(&gacha_money.Assets{}).
			Where("asset_no = ? AND state = ?", assetNo, assetStateDelivered).
			Update("state", assetStateCanceled)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			statusOut = "skipped"
			l.Infof("asset already canceled, assetNo: %s, skip", assetNo)
			return nil
		}
		return nil
	})
	if err != nil {
		l.Errorf("GrantAssetRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（branch 已成功/重试/重复补偿）：读库如实返回
		return l.readBackAsset(assetNo)
	}
	l.Infof("GrantAssetRevert success, requestId: %s, status: %s", in.RequestId, statusOut)
	return &money.GrantAssetResp{Status: statusOut}, nil
}

// readBackAsset 屏障跳过时读库如实返回当前状态
func (l *GrantAssetRevertLogic) readBackAsset(assetNo string) (*money.GrantAssetResp, error) {
	a, err := l.svcCtx.AssetModel.FindOneByAssetNo(l.ctx, assetNo)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			l.Infof("readBackAsset: asset not found, assetNo: %s, treat as skipped", assetNo)
			return &money.GrantAssetResp{Status: "skipped"}, nil
		}
		l.Errorf("readBackAsset failed, assetNo: %s, err: %v", assetNo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackAsset: asset state %d, assetNo: %s", a.State, assetNo)
	if a.State == assetStateCanceled {
		return &money.GrantAssetResp{AssetId: a.Id, ItemName: a.ItemName, Status: "canceled"}, nil
	}
	return &money.GrantAssetResp{AssetId: a.Id, ItemName: a.ItemName, Status: "skipped"}, nil
}
