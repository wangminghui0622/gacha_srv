// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package campaign

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/campaign"
	"gacha_srv/app/gateway/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 我的活动宝箱资格（M11）
func ListCampaignBoxesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := campaign.NewListCampaignBoxesLogic(r.Context(), svcCtx)
		resp, err := l.ListCampaignBoxes()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
