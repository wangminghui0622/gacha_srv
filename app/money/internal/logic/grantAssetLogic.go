package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/draw/drawClient"
	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// errAssetDuplicated 事务内哨兵：同 asset_no 的资产已存在（并发重复请求）
var errAssetDuplicated = errors.New("duplicated asset request")

type GrantAssetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantAssetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantAssetLogic {
	return &GrantAssetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantAsset SAGA 正向分支 06：开奖后发资产（asset DELIVERED，asset_no="a_"+requestId）
//
// 错误语义（SAGA 分支铁律）：
//   - 业务失败（无开奖记录）→ codes.Aborted，DTM 逆序补偿（撤销 05 开奖）
//   - 系统错误 → codes.Internal，DTM 退避重试
//
// 谢谢参与（isPrize=false）不落资产，分支直接成功。
func (l *GrantAssetLogic) GrantAsset(in *money.GrantAssetReq) (*money.GrantAssetResp, error) {
	if in.UserId <= 0 || in.BoxId <= 0 || in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=userId/boxId/requestId不能为空")
	}
	assetNo := "a_" + in.RequestId

	// 反查开奖结果（只读，放 barrier 外：DTM 重试重复调用无害）
	rec, err := l.svcCtx.DrawRpc.GetRecord(l.ctx, &drawClient.GetRecordReq{
		RequestId: in.RequestId,
		UserId:    in.UserId,
	})
	if err != nil {
		var ce *errorx.CodeError
		if errors.As(err, &ce) {
			// 下游业务码透传：Aborted 触发补偿
			l.Errorf("GrantAsset GetRecord business failed, code: %d, msg: %s", ce.Code, ce.Msg)
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
		}
		l.Errorf("GrantAsset GetRecord failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if rec == nil || rec.Record == nil {
		l.Errorf("GrantAsset record not found, requestId: %s", in.RequestId)
		return nil, status.Error(codes.Aborted, "code=3004 msg=开奖记录不存在")
	}
	if !rec.Record.IsPrize {
		// 谢谢参与：不落资产，分支直接成功
		l.Infof("GrantAsset no prize, requestId: %s, skip asset", in.RequestId)
		return &money.GrantAssetResp{AssetId: 0, ItemName: rec.Record.EntryName, Status: "granted"}, nil
	}
	itemName := rec.Record.EntryName

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("GrantAsset get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("GrantAsset get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var assetId int64
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)
		// 先查后插：撞键（DTM 重试/并发重复）读回已发放资产如实返回
		var existing gacha_money.Assets
		err := db.Where("asset_no = ?", assetNo).First(&existing).Error
		if err == nil {
			assetId = existing.Id
			l.Infof("GrantAsset idempotent: asset exists, assetNo: %s", assetNo)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := time.Now()
		asset := &gacha_money.Assets{
			AssetNo:   assetNo,
			UserId:    in.UserId,
			BoxId:     in.BoxId,
			RecordId:  rec.Record.RecordId,
			ItemName:  itemName,
			State:     assetStateDelivered,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := l.svcCtx.AssetModel.Insert(l.ctx, db, asset); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				// 并发同单重复：先到者已插入（本事务回滚），读回返回
				return errAssetDuplicated
			}
			return err
		}
		assetId = asset.Id
		return nil
	})
	if err != nil {
		if errors.Is(err, errAssetDuplicated) {
			return l.readBackAsset(assetNo)
		}
		l.Errorf("GrantAsset failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（branch 已成功/重试/重复补偿）：读库如实返回
		return l.readBackAsset(assetNo)
	}
	l.Infof("GrantAsset success, userId: %d, assetNo: %s, item: %s", in.UserId, assetNo, itemName)
	return &money.GrantAssetResp{AssetId: assetId, ItemName: itemName, Status: "granted"}, nil
}

// readBackAsset 屏障跳过/撞键时读回资产如实返回
func (l *GrantAssetLogic) readBackAsset(assetNo string) (*money.GrantAssetResp, error) {
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
	return &money.GrantAssetResp{AssetId: a.Id, ItemName: a.ItemName, Status: "granted"}, nil
}
