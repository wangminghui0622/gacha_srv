package middleware

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/pkg/utils/errorx"
)

// RequestIdMiddleware X-Request-Id 中间件（server_v2 console 要求所有控制台请求
// 携带 X-Request-Id，用于贯穿网关 → ops-rpc 授权决策 → 审计日志）。
type RequestIdMiddleware struct {
}

func NewRequestIdMiddleware() *RequestIdMiddleware {
	return &RequestIdMiddleware{}
}

func (m *RequestIdMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" || len(rid) > 128 {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeConsoleRequestIdMissing, "缺少 X-Request-Id 请求头"))
			return
		}
		w.Header().Set("X-Request-Id", rid)
		next(w, r.WithContext(restx.WithRequestId(r.Context(), rid)))
	}
}
