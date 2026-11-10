// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PutLoyaltyDefaultsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPutLoyaltyDefaultsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutLoyaltyDefaultsLogic {
	return &PutLoyaltyDefaultsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PutLoyaltyDefaults 透传 game-rpc（loyalty_offset_config upsert）。
func (l *PutLoyaltyDefaultsLogic) PutLoyaltyDefaults(in *marketing.PutTenantLoyaltyDefaultsRequest) (*marketing.TenantLoyaltyDefaults, error) {
	return l.svcCtx.MarketingRpc.PutTenantLoyaltyDefaults(l.ctx, in)
}
