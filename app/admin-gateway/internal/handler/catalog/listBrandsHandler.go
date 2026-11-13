package catalog

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/catalog"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"
)

// 品牌列表（catalog.product:read；分页 + 名称搜索）。
func ListBrandsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		req := &catalogpb.ListBrandsRequest{
			Keyword:  restx.QueryString(r, "keyword"),
			Page:     page,
			PageSize: pageSize,
		}
		resp, err := catalog.NewListBrandsLogic(r.Context(), svcCtx).ListBrands(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
