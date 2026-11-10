// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package catalog

import (
	"net/http"

	catalogpb "gacha_srv/app/catalog/catalog"
	"gacha_srv/app/admin-gateway/internal/logic/catalog"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// PublishProduct 首次发布商品（catalog.product:publish，透传 catalog-rpc）。
func PublishProductHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productId, err := restx.PathUint64(r, "productId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req catalogpb.PublishProductRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.ProductId = productId
		resp, err := catalog.NewPublishProductLogic(r.Context(), svcCtx).PublishProduct(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
