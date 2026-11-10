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

type CampaignPreviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCampaignPreviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CampaignPreviewLogic {
	return &CampaignPreviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CampaignPreview 参与预览（kind 分发）
func (l *CampaignPreviewLogic) CampaignPreview(req *types.CampaignPreviewReq) (resp *types.CampaignPreviewResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.MarketingRpc.CampaignPreview(l.ctx, &marketingClient.CampaignPreviewReq{
		UserId:      userId,
		CampaignKey: req.CampaignKey,
		Code:        req.Code,
	})
	if err != nil {
		l.Errorf("CampaignPreview failed, campaignKey: %s, err: %v", req.CampaignKey, err)
		return nil, errorx.FromError(err)
	}
	resp = &types.CampaignPreviewResp{
		CampaignKey:         r.CampaignKey,
		Kind:                r.Kind,
		AlreadyParticipated: r.AlreadyParticipated,
		Streak:              r.Streak,
		NextStreak:          r.NextStreak,
		SpinsLeft:           r.SpinsLeft,
		SpinDailyLimit:      r.SpinDailyLimit,
		RedemptionTimes:     r.RedemptionTimes,
		RedemptionUserLimit: r.RedemptionUserLimit,
		PreviewPrizes:       prizeItemsOf(r.PreviewPrizes),
	}
	for _, s := range r.Slots {
		resp.Slots = append(resp.Slots, types.CampaignWheelSlot{
			SlotIndex: s.SlotIndex,
			Label:     s.Label,
			Weight:    s.Weight,
			Remaining: s.Remaining,
			Prize:     prizeItemOf(s.Prize),
		})
	}
	return resp, nil
}

// prizeItemOf rpc PrizeItem → 网关 PrizeItem
func prizeItemOf(p *marketingClient.PrizeItem) types.CampaignPrizeItem {
	if p == nil {
		return types.CampaignPrizeItem{}
	}
	return types.CampaignPrizeItem{
		Kind:             p.Kind,
		Amount:           p.Amount,
		Points:           p.Points,
		CouponTemplateNo: p.CouponTemplateNo,
		CouponName:       p.CouponName,
		BoxId:            p.BoxId,
		BoxName:          p.BoxName,
	}
}

func prizeItemsOf(ps []*marketingClient.PrizeItem) []types.CampaignPrizeItem {
	items := make([]types.CampaignPrizeItem, 0, len(ps))
	for _, p := range ps {
		items = append(items, prizeItemOf(p))
	}
	return items
}
