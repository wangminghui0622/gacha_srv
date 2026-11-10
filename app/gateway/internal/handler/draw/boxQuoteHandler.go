// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package draw

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/draw"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 购买页报价（M10）：宝箱原价 + 券/积分抵扣试算（服务端算价，纯预览）
func BoxQuoteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BoxQuoteReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := draw.NewBoxQuoteLogic(r.Context(), svcCtx)
		resp, err := l.BoxQuote(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
