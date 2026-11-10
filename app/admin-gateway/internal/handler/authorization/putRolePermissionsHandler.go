package authorization

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/authorization"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 配置角色权限（authorization.role:write，整表替换 + 版本自增）
func PutRolePermissionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ops.PutRolePermissionsRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		roleId, err := restx.PathUint64(r, "roleId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req.RoleId = roleId
		resp, err := authorization.NewPutRolePermissionsLogic(r.Context(), svcCtx).PutRolePermissions(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
