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

func ListConsoleUsersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := restx.QueryUint32(r, "page")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		status, err := restx.QueryUint32(r, "status")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := customer.NewListConsoleUsersLogic(r.Context(), svcCtx).ListConsoleUsers(&identity.ListConsoleUsersReq{
			TenantId: 1,
			Keyword:  restx.QueryString(r, "keyword"),
			Phone:    restx.QueryString(r, "phone"),
			Identity: restx.QueryString(r, "identity"),
			Status:   int32(status),
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
