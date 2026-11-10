// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package campaign

import (
	"net/http"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/logic/campaign"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// PublishCampaign 发布活动（campaign.definition:publish；发布最新 DRAFT 版本；
// campaignId 取路径，tenant 固定 1）。
func PublishCampaignHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		campaignId, err := restx.PathUint64(r, "campaignId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req marketing.PublishCampaignRequest
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.CampaignId = campaignId
		req.TenantId = 1
		resp, err := campaign.NewPublishCampaignLogic(r.Context(), svcCtx).PublishCampaign(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
