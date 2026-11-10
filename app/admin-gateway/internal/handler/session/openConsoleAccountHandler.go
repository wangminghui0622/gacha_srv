package session

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/session"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 开通控制台账号（identity.console_account:create）
func OpenConsoleAccountHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ops.OpenConsoleAccountRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := session.NewOpenConsoleAccountLogic(r.Context(), svcCtx).OpenConsoleAccount(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
