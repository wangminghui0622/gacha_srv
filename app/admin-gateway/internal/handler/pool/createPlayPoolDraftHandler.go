// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/pool"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"
)

// CreatePlayPoolDraftHandler 创建统一玩赏池草稿（gacha.pool:create；Idempotency-Key 头必填）。
func CreatePlayPoolDraftHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req play.CreatePlayPoolDraftReq
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
		resp, err := pool.NewCreatePlayPoolDraftLogic(r.Context(), svcCtx).CreatePlayPoolDraft(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
