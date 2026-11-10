package authorization

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/authorization"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 配置主体角色（authorization.role:write，expectedVersion 乐观锁）
func PutPrincipalRolesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ops.PutPrincipalRolesRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		principalId, err := restx.PathUint64(r, "principalId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req.PrincipalId = principalId
		resp, err := authorization.NewPutPrincipalRolesLogic(r.Context(), svcCtx).PutPrincipalRoles(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
