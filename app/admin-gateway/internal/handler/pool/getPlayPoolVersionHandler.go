// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"
)

// GetPlayPoolVersionHandler 统一玩赏池版本详情（gacha.pool:read）。
func GetPlayPoolVersionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		poolId, err := restx.PathUint64(r, "poolId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		version, err := restx.PathUint64(r, "version")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &play.GetPlayPoolVersionReq{PoolId: int64(poolId), Version: int64(version)}
		resp, err := pool.NewGetPlayPoolVersionLogic(r.Context(), svcCtx).GetPlayPoolVersion(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
