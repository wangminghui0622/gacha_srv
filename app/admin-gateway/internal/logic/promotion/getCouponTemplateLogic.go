// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCouponTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCouponTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCouponTemplateLogic {
	return &GetCouponTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCouponTemplate 透传 game-rpc（最新已发布版本）。
func (l *GetCouponTemplateLogic) GetCouponTemplate(in *marketing.GetCouponTemplateRequest) (*marketing.CouponTemplate, error) {
	return l.svcCtx.MarketingRpc.GetCouponTemplate(l.ctx, in)
}
