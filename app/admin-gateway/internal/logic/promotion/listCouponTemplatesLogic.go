// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCouponTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCouponTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCouponTemplatesLogic {
	return &ListCouponTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListCouponTemplates 透传 marketing-rpc（模板版本游标分页）。
func (l *ListCouponTemplatesLogic) ListCouponTemplates(in *marketing.ListCouponTemplateVersionsRequest) (*marketing.ListCouponTemplateVersionsReply, error) {
	return l.svcCtx.MarketingRpc.ListCouponTemplateVersions(l.ctx, in)
}
