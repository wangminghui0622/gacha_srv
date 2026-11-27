// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"net/http"
	"strings"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"
)

// VoidPlayPoolHandler 作废赏池（gacha.pool:publish）；reason 走 query 参数。
func VoidPlayPoolHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		poolId, err := restx.PathUint64(r, "poolId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &play.VoidPlayPoolReq{
			PoolId: int64(poolId),
			Reason: strings.TrimSpace(r.URL.Query().Get("reason")),
		}
		resp, err := pool.NewVoidPlayPoolLogic(r.Context(), svcCtx).VoidPlayPool(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
