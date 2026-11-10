// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package campaign

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishCampaignLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishCampaignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishCampaignLogic {
	return &PublishCampaignLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishCampaign 透传 marketing-rpc（发布最新 DRAFT 版本）。
func (l *PublishCampaignLogic) PublishCampaign(in *marketing.PublishCampaignRequest) (*marketing.Campaign, error) {
	return l.svcCtx.MarketingRpc.PublishCampaign(l.ctx, in)
}
