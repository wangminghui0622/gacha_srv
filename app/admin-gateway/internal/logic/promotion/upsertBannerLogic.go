// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertBannerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertBannerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertBannerLogic {
	return &UpsertBannerLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertBannerLogic) UpsertBanner(in *marketing.UpsertBannerRequest) (*marketing.Banner, error) {
	return l.svcCtx.MarketingRpc.UpsertBanner(l.ctx, in)
}
