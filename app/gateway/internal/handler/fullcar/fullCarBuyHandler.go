// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package fullcar

import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/fullcar"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 满车购座（M12b：提交 DTM SAGA 事务，异步占座开奖，客户端轮询记录）
func FullCarBuyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.FullCarBuyReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := fullcar.NewFullCarBuyLogic(r.Context(), svcCtx)
		// Idempotency-Key 头 → SAGA gid：客户端超时重试带同一键，复用同一事务
		l.SetRequestId(r.Header.Get("Idempotency-Key"))
		resp, err := l.FullCarBuy(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
