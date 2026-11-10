package campaign

import (
	"context"

	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCampaignsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListCampaignsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCampaignsLogic {
	return &ListCampaignsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListCampaigns 活动列表 + 我的状态
func (l *ListCampaignsLogic) ListCampaigns() (resp *types.CampaignListResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.MarketingRpc.ListCampaigns(l.ctx, &marketingClient.ListCampaignsReq{UserId: userId})
	if err != nil {
		l.Errorf("ListCampaigns failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	resp = &types.CampaignListResp{List: make([]types.CampaignItem, 0, len(r.Items))}
	for _, it := range r.Items {
		resp.List = append(resp.List, types.CampaignItem{
			CampaignKey:    it.CampaignKey,
			Name:           it.Name,
			Kind:           it.Kind,
			StartAt:        it.StartAt,
			EndAt:          it.EndAt,
			InWindow:       it.InWindow,
			SignedToday:    it.SignedToday,
			Streak:         it.Streak,
			TotalDays:      it.TotalDays,
			SpinsLeft:      it.SpinsLeft,
			SpinDailyLimit: it.SpinDailyLimit,
		})
	}
	return resp, nil
}
