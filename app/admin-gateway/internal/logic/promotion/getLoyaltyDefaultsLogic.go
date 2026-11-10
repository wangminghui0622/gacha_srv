// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLoyaltyDefaultsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLoyaltyDefaultsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLoyaltyDefaultsLogic {
	return &GetLoyaltyDefaultsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetLoyaltyDefaults 透传 game-rpc（无配置行时返回平台默认）。
func (l *GetLoyaltyDefaultsLogic) GetLoyaltyDefaults(in *marketing.GetTenantLoyaltyDefaultsRequest) (*marketing.TenantLoyaltyDefaults, error) {
	return l.svcCtx.MarketingRpc.GetTenantLoyaltyDefaults(l.ctx, in)
}
