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

// GetCampaign 查看活动（campaign.definition:read；tenant 固定 1）。
func GetCampaignHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		campaignId, err := restx.PathUint64(r, "campaignId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &marketing.GetCampaignRequest{CampaignId: campaignId, TenantId: 1}
		resp, err := campaign.NewGetCampaignLogic(r.Context(), svcCtx).GetCampaign(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
