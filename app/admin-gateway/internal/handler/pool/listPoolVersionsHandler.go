// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"net/http"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// ListPoolVersions 赏池版本列表（gacha.pool:read；游标分页）。
func ListPoolVersionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		afterPoolId, err := restx.QueryUint64(r, "afterPoolId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		afterVersion, err := restx.QueryUint64(r, "afterVersion")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint64(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &draw.ListPoolVersionsReq{
			AfterPoolId:  int64(afterPoolId),
			AfterVersion: int64(afterVersion),
			PageSize:     int32(pageSize),
		}
		resp, err := pool.NewListPoolVersionsLogic(r.Context(), svcCtx).ListPoolVersions(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
