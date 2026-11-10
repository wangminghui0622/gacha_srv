// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package groupbuy

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/groupbuy"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 拼团抽详情（M13：余位/我的下注/参与人数/幸运奖进度）
func GetGroupBuyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GroupBuyPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := groupbuy.NewGetGroupBuyLogic(r.Context(), svcCtx)
		resp, err := l.GetGroupBuy(req.ConfigId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
