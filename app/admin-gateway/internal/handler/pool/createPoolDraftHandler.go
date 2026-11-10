// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"net/http"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"
)

// CreatePoolDraft 创建赏池草稿（gacha.pool:create；幂等键走 Idempotency-Key 头，
// 缺失 → 400 IDEMPOTENCY_KEY_REQUIRED，与 server_v2 一致）。
func CreatePoolDraftHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req draw.CreatePoolDraftReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.IdempotencyKey = r.Header.Get("Idempotency-Key")
		if req.IdempotencyKey == "" {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeConsoleIdempotencyRequired, "缺少 Idempotency-Key 头"))
			return
		}
		resp, err := pool.NewCreatePoolDraftLogic(r.Context(), svcCtx).CreatePoolDraft(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
