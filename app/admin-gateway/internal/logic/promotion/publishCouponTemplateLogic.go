// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishCouponTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishCouponTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishCouponTemplateLogic {
	return &PublishCouponTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishCouponTemplate 透传 marketing-rpc（requestId 幂等：同号同内容重放返回已发布版本）。
func (l *PublishCouponTemplateLogic) PublishCouponTemplate(in *marketing.PublishCouponTemplateRequest) (*marketing.CouponTemplate, error) {
	return l.svcCtx.MarketingRpc.PublishCouponTemplate(l.ctx, in)
}
