package catalog

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/catalog"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"
)

// IP 列表（catalog.product:read；分页 + 名称搜索）。
func ListIPsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		req := &catalogpb.ListIPsRequest{
			Keyword:  restx.QueryString(r, "keyword"),
			Page:     page,
			PageSize: pageSize,
		}
		resp, err := catalog.NewListIPsLogic(r.Context(), svcCtx).ListIPs(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
