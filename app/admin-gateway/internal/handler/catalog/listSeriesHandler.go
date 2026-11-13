package catalog

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/catalog"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"
)

// 系列列表（catalog.product:read；分页 + 名称搜索 + 品牌筛选）。
func ListSeriesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		req := &catalogpb.ListSeriesRequest{
			Keyword:  restx.QueryString(r, "keyword"),
			BrandId:  brandID,
			Page:     page,
			PageSize: pageSize,
		}
		resp, err := catalog.NewListSeriesLogic(r.Context(), svcCtx).ListSeries(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
