package authorization

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/authorization"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 角色成员（authorization.role:read；游标分页 afterPrincipalId）
func ListRoleMembersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		afterPrincipalId, err := restx.QueryUint64(r, "afterPrincipalId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &ops.ListRoleMembersRequest{TenantId: tenantId, RoleId: roleId, PageSize: pageSize, AfterPrincipalId: afterPrincipalId}
		resp, err := authorization.NewListRoleMembersLogic(r.Context(), svcCtx).ListRoleMembers(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
