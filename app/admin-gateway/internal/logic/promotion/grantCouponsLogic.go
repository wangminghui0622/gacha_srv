// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GrantCouponsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantCouponsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantCouponsLogic {
	return &GrantCouponsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantCoupons 透传 game-rpc（发最新已发布版本；requestId 幂等）。
func (l *GrantCouponsLogic) GrantCoupons(in *marketing.GrantCouponsRequest) (*marketing.GrantCouponsReply, error) {
	return l.svcCtx.MarketingRpc.GrantCoupons(l.ctx, in)
}
