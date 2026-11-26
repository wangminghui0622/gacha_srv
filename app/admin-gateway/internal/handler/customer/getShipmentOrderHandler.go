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

// GetShipmentOrder 发货订单详情（identity.user:read）。
func GetShipmentOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := restx.PathUint64(r, "id")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := customer.NewGetShipmentOrderLogic(r.Context(), svcCtx).GetShipmentOrder(&draw.GetShipmentOrderReq{Id: int64(id)})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
