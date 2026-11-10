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

// 手机号登录验证码（M15a，公开）
func PhoneCodeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PhoneCodeReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := identity.NewPhoneCodeLogic(r.Context(), svcCtx)
		resp, err := l.PhoneCode(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
