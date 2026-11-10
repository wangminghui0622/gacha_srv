// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package identity
import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/identity"
	"gacha_srv/app/gateway/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 余额查询
func BalanceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := identity.NewBalanceLogic(r.Context(), svcCtx)
		resp, err := l.Balance()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
