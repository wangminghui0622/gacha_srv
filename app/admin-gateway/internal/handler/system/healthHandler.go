package system

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/system"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 健康检查（公开，无中间件）
func HealthHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := system.NewHealthLogic(r.Context(), svcCtx).Health()
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteJSON(w, r, resp)
	}
}
