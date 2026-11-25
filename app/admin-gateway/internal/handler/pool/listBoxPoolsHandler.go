// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/draw/draw"
)

// ListBoxPools 无限赏宝箱列表（gacha.pool:read；每个赏池仅返回最新版本）。
func ListBoxPoolsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := restx.QueryUint64(r, "page")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint64(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &draw.ListBoxPoolsReq{
			Page:     int32(page),
			PageSize: int32(pageSize),
			Keyword:  r.URL.Query().Get("keyword"),
		}
		resp, err := pool.NewListBoxPoolsLogic(r.Context(), svcCtx).ListBoxPools(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
