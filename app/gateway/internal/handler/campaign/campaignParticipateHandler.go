// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package campaign

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/campaign"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 活动统一参与（M11）：强制 Idempotency-Key 头（缺失/超长 → 400）
func CampaignParticipateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CampaignParticipateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := campaign.NewCampaignParticipateLogic(r.Context(), svcCtx, r.Header.Get("Idempotency-Key"))
		resp, err := l.CampaignParticipate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
