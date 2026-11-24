package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetCouponPoolEntryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCouponPoolEntryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCouponPoolEntryLogic {
	return &GetCouponPoolEntryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCouponPoolEntry 查看单条券池记录。
func (l *GetCouponPoolEntryLogic) GetCouponPoolEntry(in *marketing.GetCouponPoolEntryRequest) (*marketing.CouponPoolEntry, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId 须为 1（单租户）")
	}
	if in.Id == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "id 必须大于 0")
	}
	row, err := repo.GetCouponPoolEntry(l.svcCtx.DB, int64(in.Id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeCouponPoolNotFound, "券池记录不存在")
		}
		return nil, err
	}
	return row.ToProto(), nil
}
