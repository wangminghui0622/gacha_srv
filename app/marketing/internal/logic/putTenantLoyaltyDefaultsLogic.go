package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PutTenantLoyaltyDefaultsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPutTenantLoyaltyDefaultsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutTenantLoyaltyDefaultsLogic {
	return &PutTenantLoyaltyDefaultsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PutTenantLoyaltyDefaults 设计 D5：写 loyalty_offset_config id=1。
// 校验：tenant 单租户固定 1；loyalty_points_per_major>0；0<max_gross_bps≤10000。
func (l *PutTenantLoyaltyDefaultsLogic) PutTenantLoyaltyDefaults(in *marketing.PutTenantLoyaltyDefaultsRequest) (*marketing.TenantLoyaltyDefaults, error) {
	d := in.Defaults
	if d == nil {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "defaults 不能为空")
	}
	if d.TenantId != 0 && d.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId 须为 1（单租户）")
	}
	if d.LoyaltyPointsPerMajor == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "loyaltyPointsPerMajor 必须大于 0")
	}
	if d.MaxGrossBps == 0 || d.MaxGrossBps > 10000 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "maxGrossBps 须在 (0, 10000]")
	}
	if err := repo.PutLoyaltyDefaults(l.svcCtx.DB, int64(d.LoyaltyPointsPerMajor), int64(d.MaxGrossBps)); err != nil {
		return nil, err
	}
	return &marketing.TenantLoyaltyDefaults{
		TenantId:              1,
		LoyaltyPointsPerMajor: d.LoyaltyPointsPerMajor,
		MaxGrossBps:           d.MaxGrossBps,
	}, nil
}
