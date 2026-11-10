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

// GetProduct 查看商品（catalog.product:read，透传 catalog-rpc）。
func GetProductHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productId, err := restx.PathUint64(r, "productId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &catalogpb.GetProductRequest{ProductId: productId}
		resp, err := catalog.NewGetProductLogic(r.Context(), svcCtx).GetProduct(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
