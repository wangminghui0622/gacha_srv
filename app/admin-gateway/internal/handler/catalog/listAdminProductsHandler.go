package catalog

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/catalog"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"
)

// 商品管理列表（catalog.product:read；多条件筛选 + 分页）。
func ListAdminProductsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		brandID, err := restx.QueryUint64(r, "brandId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		seriesID, err := restx.QueryUint64(r, "seriesId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		ipID, err := restx.QueryUint64(r, "ipId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		minPrice, err := restx.QueryUint64(r, "minPriceMinor")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		maxPrice, err := restx.QueryUint64(r, "maxPriceMinor")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		status, err := restx.QueryEnum(r, "status", catalogpb.ProductStatus_value)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &catalogpb.ListAdminProductsRequest{
			Keyword:       restx.QueryString(r, "keyword"),
			BrandId:       brandID,
			SeriesId:      seriesID,
			IpId:          ipID,
			SaleType:      restx.QueryString(r, "saleType"),
			MinPriceMinor: int64(minPrice),
			MaxPriceMinor: int64(maxPrice),
			Status:        catalogpb.ProductStatus(status),
			SortBy:        restx.QueryString(r, "sortBy"),
			SortOrder:     restx.QueryString(r, "sortOrder"),
			Page:          page,
			PageSize:      pageSize,
		}
		resp, err := catalog.NewListAdminProductsLogic(r.Context(), svcCtx).ListAdminProducts(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
