package authorization

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/authorization"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 查看角色（authorization.role:read）
func GetRoleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roleId, err := restx.PathUint64(r, "roleId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		tenantId, err := restx.QueryUint64(r, "tenantId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := authorization.NewGetRoleLogic(r.Context(), svcCtx).GetRole(&ops.GetRoleRequest{TenantId: tenantId, RoleId: roleId})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
