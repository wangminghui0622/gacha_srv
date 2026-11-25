// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBannersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBannersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBannersLogic {
	return &ListBannersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListBannersLogic) ListBanners(in *marketing.ListBannersRequest) (*marketing.ListBannersReply, error) {
	return l.svcCtx.MarketingRpc.ListBanners(l.ctx, in)
}
