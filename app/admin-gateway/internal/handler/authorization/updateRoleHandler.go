package authorization

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/authorization"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 修改角色（authorization.role:write，expectedVersion 乐观锁）
func UpdateRoleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ops.UpdateRoleRequest
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
		resp, err := authorization.NewUpdateRoleLogic(r.Context(), svcCtx).UpdateRole(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
