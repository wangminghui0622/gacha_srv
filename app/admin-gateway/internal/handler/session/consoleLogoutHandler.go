package session

import (
	"net/http"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/logic/session"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// 退出登录（吊销指定会话；sessionId 缺省取当前 JWT 的 jti）
func ConsoleLogoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ops.ConsoleLogoutRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if req.SessionId == "" {
			if claims := restx.ClaimsFrom(r.Context()); claims != nil {
				req.SessionId = claims.SessionID
			}
		}
		resp, err := session.NewConsoleLogoutLogic(r.Context(), svcCtx).ConsoleLogout(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
