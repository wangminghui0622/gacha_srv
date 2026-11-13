package catalog

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/catalog"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"
)

// 编辑品牌（catalog.product:write）。
func UpdateBrandHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req catalogpb.UpdateBrandRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		brandID, err := restx.PathUint64(r, "brandId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req.BrandId = brandID
		resp, err := catalog.NewUpdateBrandLogic(r.Context(), svcCtx).UpdateBrand(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
