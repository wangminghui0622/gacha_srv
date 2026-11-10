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

// 活动参与预览（M11）
func CampaignPreviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CampaignPreviewReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := campaign.NewCampaignPreviewLogic(r.Context(), svcCtx)
		resp, err := l.CampaignPreview(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
