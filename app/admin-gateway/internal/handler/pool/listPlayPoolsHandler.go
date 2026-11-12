// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"
)

// ListPlayPoolsHandler 统一玩赏池列表（gacha.pool:read；游标分页）。
func ListPlayPoolsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		afterPoolId, err := restx.QueryUint64(r, "afterPoolId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint64(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &play.ListPlayPoolsReq{
			AfterPoolId: int64(afterPoolId),
			PageSize:    int64(pageSize),
			Mode:        r.URL.Query().Get("mode"),
		}
		resp, err := pool.NewListPlayPoolsLogic(r.Context(), svcCtx).ListPlayPools(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
