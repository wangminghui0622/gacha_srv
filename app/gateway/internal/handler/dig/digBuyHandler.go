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

// 挖宝购买（提交 DTM SAGA 事务，异步开格，客户端轮询记录，M11b）
func DigBuyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DigBuyReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := dig.NewDigBuyLogic(r.Context(), svcCtx)
		// Idempotency-Key 头 → SAGA gid：客户端超时重试带同一键，复用同一事务
		l.SetRequestId(r.Header.Get("Idempotency-Key"))
		resp, err := l.DigBuy(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
