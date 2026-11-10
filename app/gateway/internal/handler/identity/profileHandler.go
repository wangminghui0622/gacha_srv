// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package identity
import (
	"net/http"

	"gacha_srv/app/gateway/internal/logic/identity"
	"gacha_srv/app/gateway/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// 用户资料（含余额）
func ProfileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := identity.NewProfileLogic(r.Context(), svcCtx)
		resp, err := l.Profile()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
