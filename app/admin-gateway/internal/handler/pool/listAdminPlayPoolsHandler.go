package pool

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"
)

// 赏池管理列表（gacha.pool:read；多条件筛选 + 分页）。
func ListAdminPlayPoolsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		ipID, err := restx.QueryUint64(r, "ipId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &play.ListAdminPlayPoolsReq{
			Keyword:   restx.QueryString(r, "keyword"),
			Mode:      restx.QueryString(r, "mode"),
			IpId:      ipID,
			Status:    restx.QueryString(r, "status"),
			Tag:       restx.QueryString(r, "tag"),
			SortOrder: restx.QueryString(r, "sortOrder"),
			Page:      page,
			PageSize:  pageSize,
		}
		resp, err := pool.NewListAdminPlayPoolsLogic(r.Context(), svcCtx).ListAdminPlayPools(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
