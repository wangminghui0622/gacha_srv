package logic

import (
	"context"
	"errors"
	"math"
	"time"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type AdjustCouponPoolEntryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdjustCouponPoolEntryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdjustCouponPoolEntryLogic {
	return &AdjustCouponPoolEntryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AdjustCouponPoolEntry 管理员增加/减少券池库存：行锁 + 原子更新 + 写流水。
func (l *AdjustCouponPoolEntryLogic) AdjustCouponPoolEntry(in *marketing.AdjustCouponPoolEntryRequest) (*marketing.CouponPoolEntry, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId 须为 1（单租户）")
	}
	if in.EntryId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "entryId 必须大于 0")
	}
	if in.Delta == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "调整数量不能为 0")
	}
	operatorType := in.OperatorType
	if operatorType == "" {
		operatorType = "ADMIN"
	}

	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		row, err := repo.LockCouponPoolEntry(tx, int64(in.EntryId))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorx.NewCodeError(errorx.CodeCouponPoolNotFound, "券池记录不存在")
			}
			return err
		}
		granted, err := repo.SumGrantedCouponCount(tx, row.ID)
		if err != nil {
			return err
		}
		afterRemaining := int64(row.RemainingCount) + int64(in.Delta)
		if afterRemaining < 0 {
			return errorx.NewCodeErrorf(errorx.CodeCouponInvalid, "库存不足：当前剩余 %d，减少 %d", row.RemainingCount, -in.Delta)
		}
		// 总张数 = 剩余 + 已发出张数（GRANT 流水累计），不依赖历史 count 快照。
		afterTotal := afterRemaining + granted
		if afterRemaining > math.MaxInt32 || afterTotal > math.MaxInt32 {
			return errorx.NewCodeError(errorx.CodeCouponInvalid, "调整后库存超出上限")
		}
		if err := repo.UpdateCouponPoolStock(tx, row.ID, int32(afterTotal), int32(afterRemaining)); err != nil {
			return err
		}
		eventType := "ADMIN_ADD"
		if in.Delta < 0 {
			eventType = "ADMIN_SUB"
		}
		ledger := &repo.CouponLedgerRow{
			EntryID:      row.ID,
			EventType:    eventType,
			Delta:        in.Delta,
			BeforeCount:  row.RemainingCount,
			AfterCount:   int32(afterRemaining),
			OperatorType: operatorType,
			OperatorID:   int64(in.OperatorId),
			Remark:       in.Remark,
			CreatedAt:    time.Now(),
		}
		return repo.InsertCouponLedger(tx, ledger)
	})
	if err != nil {
		return nil, err
	}
	saved, err := repo.GetCouponPoolEntry(l.svcCtx.DB, int64(in.EntryId))
	if err != nil {
		return nil, err
	}
	return saved.ToProto(), nil
}
