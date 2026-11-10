// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package campaign

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/campaign"
	"gacha_srv/app/gateway/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 活动列表 + 我的状态（M11）
func ListCampaignsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := campaign.NewListCampaignsLogic(r.Context(), svcCtx)
		resp, err := l.ListCampaigns()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
