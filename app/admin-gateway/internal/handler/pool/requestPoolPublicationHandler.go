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

// RequestPoolPublication 提请发布赏池（gacha.pool:request_publication；
// 申请人 = 会话主体，不接受客户端传入）。
func RequestPoolPublicationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		var req draw.RequestPoolPublicationReq
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.PoolId = int64(poolId)
		req.Version = int64(version)
		resp, err := pool.NewRequestPoolPublicationLogic(r.Context(), svcCtx).RequestPoolPublication(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
