package session

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/session"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 重置账号 TOTP（identity.console_account:reset_totp；同时吊销该账号全部活跃会话）
func ResetConsoleTotpHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ops.ResetConsoleTotpRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		principalId, err := restx.PathUint64(r, "principalId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req.PrincipalId = principalId
		resp, err := session.NewResetConsoleTotpLogic(r.Context(), svcCtx).ResetConsoleTotp(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
