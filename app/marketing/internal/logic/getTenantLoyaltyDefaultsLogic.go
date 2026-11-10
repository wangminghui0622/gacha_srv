package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetTenantLoyaltyDefaultsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTenantLoyaltyDefaultsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTenantLoyaltyDefaultsLogic {
	return &GetTenantLoyaltyDefaultsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTenantLoyaltyDefaults 设计 D5：读写 loyalty_offset_config id=1
// （loyalty_points_per_major↔points_per_yuan、max_gross_bps↔max_ratio_bps；
// tenant_id 单租户固定 1）。行缺失（种子恒有）回 0=平台默认。
func (l *GetTenantLoyaltyDefaultsLogic) GetTenantLoyaltyDefaults(in *marketing.GetTenantLoyaltyDefaultsRequest) (*marketing.TenantLoyaltyDefaults, error) {
	row, err := repo.GetLoyaltyDefaults(l.svcCtx.DB)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &marketing.TenantLoyaltyDefaults{TenantId: 1}, nil
		}
		return nil, err
	}
	return &marketing.TenantLoyaltyDefaults{
		TenantId:              1,
		LoyaltyPointsPerMajor: uint64(row.PointsPerYuan),
		MaxGrossBps:           uint32(row.MaxRatioBps),
	}, nil
}
