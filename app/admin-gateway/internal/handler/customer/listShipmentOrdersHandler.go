// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/customer"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/draw/draw"
)

// ListShipmentOrders 发货订单列表（identity.user:read）。
func ListShipmentOrdersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := restx.QueryUint32(r, "status")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
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
		resp, err := customer.NewListShipmentOrdersLogic(r.Context(), svcCtx).ListShipmentOrders(&draw.ListShipmentOrdersReq{
			Keyword:  restx.QueryString(r, "keyword"),
			Status:   int32(status),
			Page:     int32(page),
			PageSize: int32(pageSize),
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
