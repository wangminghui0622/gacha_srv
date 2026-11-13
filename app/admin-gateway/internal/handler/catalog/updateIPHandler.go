package catalog

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/catalog"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"
)

// 编辑 IP（catalog.product:write）。
func UpdateIPHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req catalogpb.UpdateIPRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		ipID, err := restx.PathUint64(r, "ipId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req.IpId = ipID
		resp, err := catalog.NewUpdateIPLogic(r.Context(), svcCtx).UpdateIP(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
