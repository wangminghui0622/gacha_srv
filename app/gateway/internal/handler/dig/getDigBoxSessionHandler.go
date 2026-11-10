// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package dig

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/dig"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 挖宝会话（位图/已挖数/状态，M11b）
func GetDigBoxSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DigSessionReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := dig.NewGetDigBoxSessionLogic(r.Context(), svcCtx)
		resp, err := l.GetDigBoxSession(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
