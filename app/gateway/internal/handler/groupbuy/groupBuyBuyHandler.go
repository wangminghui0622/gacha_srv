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

// 拼团抽下注（M13：提交 DTM SAGA 事务，异步整批结算，客户端轮询记录）
func GroupBuyBuyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GroupBuyBuyReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := groupbuy.NewGroupBuyBuyLogic(r.Context(), svcCtx)
		// Idempotency-Key 头 → SAGA gid：客户端超时重试带同一键，复用同一事务
		l.SetRequestId(r.Header.Get("Idempotency-Key"))
		resp, err := l.GroupBuyBuy(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
