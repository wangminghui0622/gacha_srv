package session

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/session"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 读取当前控制台账号的页签（actor 来自 JWT claims，只能读取自己的页签）。
func GetConsoleTabsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := session.NewGetConsoleTabsLogic(r.Context(), svcCtx).GetConsoleTabs()
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
