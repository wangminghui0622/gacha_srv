package catalog

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/catalog"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"
)

// 创建品牌（catalog.product:create）。
func CreateBrandHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req catalogpb.CreateBrandRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := catalog.NewCreateBrandLogic(r.Context(), svcCtx).CreateBrand(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
