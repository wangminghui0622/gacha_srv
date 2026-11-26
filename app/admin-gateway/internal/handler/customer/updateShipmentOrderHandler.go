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

// UpdateShipmentOrder 编辑发货订单（identity.user:write）。
func UpdateShipmentOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := restx.PathUint64(r, "id")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req draw.UpdateShipmentOrderReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.Id = int64(id)
		resp, err := customer.NewUpdateShipmentOrderLogic(r.Context(), svcCtx).UpdateShipmentOrder(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
