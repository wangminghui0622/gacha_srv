package session

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/session"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 当前有效权限（actor 来自 JWT claims；免权限点，需会话）
func ConsolePermissionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := session.NewConsolePermissionsLogic(r.Context(), svcCtx).ConsolePermissions()
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
