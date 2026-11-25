// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/customer"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"
)

func UpdateConsoleUserHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := restx.PathUint64(r, "id")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req identity.UpdateConsoleUserReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.TenantId = 1
		req.RequestId = restx.RequestIdFrom(r.Context())
		req.UserId = int64(id)
		resp, err := customer.NewUpdateConsoleUserLogic(r.Context(), svcCtx).UpdateConsoleUser(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
