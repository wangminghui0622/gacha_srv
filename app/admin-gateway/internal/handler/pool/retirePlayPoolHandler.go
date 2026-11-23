// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"
)

// RetirePlayPoolHandler 下架赏池的所有已发布版本（gacha.pool:publish）。
func RetirePlayPoolHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		poolId, err := restx.PathUint64(r, "poolId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &play.RetirePlayPoolReq{PoolId: int64(poolId)}
		resp, err := pool.NewRetirePlayPoolLogic(r.Context(), svcCtx).RetirePlayPool(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
