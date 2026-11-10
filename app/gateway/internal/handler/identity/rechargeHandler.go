// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package identity
import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/identity"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 充值（mock，直接加款）
func RechargeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RechargeReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := identity.NewRechargeLogic(r.Context(), svcCtx)
		// Idempotency-Key 头 → 充值幂等键：客户端重试带同一键只生效一次
		l.SetRequestId(r.Header.Get("Idempotency-Key"))
		resp, err := l.Recharge(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
