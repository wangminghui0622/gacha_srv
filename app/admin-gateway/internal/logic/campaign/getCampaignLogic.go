// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package campaign

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCampaignLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCampaignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCampaignLogic {
	return &GetCampaignLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCampaign 透传 marketing-rpc。
func (l *GetCampaignLogic) GetCampaign(in *marketing.GetCampaignRequest) (*marketing.Campaign, error) {
	return l.svcCtx.MarketingRpc.GetCampaign(l.ctx, in)
}
