// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package campaign

import (
	"context"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateCampaignDraftLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCampaignDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCampaignDraftLogic {
	return &CreateCampaignDraftLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateCampaignDraft 透传 marketing-rpc（tenant 已由 handler 固定为 1）。
func (l *CreateCampaignDraftLogic) CreateCampaignDraft(in *marketing.CreateCampaignDraftRequest) (*marketing.Campaign, error) {
	return l.svcCtx.MarketingRpc.CreateCampaignDraft(l.ctx, in)
}
