package authorization

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/authorization"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 查看主体角色（authorization.role:read；version 为账号行 role_version 乐观锁）
func GetPrincipalRolesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principalId, err := restx.PathUint64(r, "principalId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		tenantId, err := restx.QueryUint64(r, "tenantId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := authorization.NewGetPrincipalRolesLogic(r.Context(), svcCtx).GetPrincipalRoles(&ops.GetPrincipalRolesRequest{TenantId: tenantId, PrincipalId: principalId})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
