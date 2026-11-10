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

// 挖宝报价预览（M11b）
func DigQuoteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DigQuoteReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := dig.NewDigQuoteLogic(r.Context(), svcCtx)
		resp, err := l.DigQuote(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
