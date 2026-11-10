package authorization

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/authorization"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 查看权限目录（authorization.role:read）
func ListPermissionCatalogHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := authorization.NewListPermissionCatalogLogic(r.Context(), svcCtx).ListPermissionCatalog()
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
