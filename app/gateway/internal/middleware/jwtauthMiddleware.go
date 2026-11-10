// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package middleware

import (
	"context"
	"net/http"
	"strings"

	identitypb "gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/identityClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// ctxKeyUserId 用户身份上下文键（logic 用 UserIdFromCtx 读取）
type ctxKey int

const ctxKeyUserId ctxKey = iota

// WithUserId 把用户身份写入上下文（JwtAuth 校验通过后）
func WithUserId(ctx context.Context, userId int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserId, userId)
}

// UserIdFromCtx logic 层从上下文读取用户身份（规范 common/api-conventions 规则 11）
func UserIdFromCtx(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxKeyUserId).(int64)
	return v, ok
}

type JwtAuthMiddleware struct {
	identityRpc identityClient.Identity
}

// NewJwtAuthMiddleware 校验令牌：调 identity 服务 VerifyToken（Etcd），
// 校验通过把 userId 写入上下文。用户身份一律以服务端校验结果为准。
func NewJwtAuthMiddleware(identityRpc identityClient.Identity) *JwtAuthMiddleware {
	return &JwtAuthMiddleware{identityRpc: identityRpc}
}

func (m *JwtAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			httpx.ErrorCtx(r.Context(), w, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录或令牌缺失"))
			return
		}
		resp, err := m.identityRpc.VerifyToken(r.Context(), &identitypb.VerifyTokenReq{Token: token})
		if err != nil {
			// ClientErrorInterceptor 已把下游业务码还原为 CodeError，
			// 经统一信封返回（1004 令牌无效 / 500 下游故障）
			httpx.ErrorCtx(r.Context(), w, errorx.FromError(err))
			return
		}
		next(w, r.WithContext(WithUserId(r.Context(), resp.UserId)))
	}
}

// bearerToken 解析 Authorization: Bearer <token>
func bearerToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	return token, token != ""
}

// OptionalHandle 可选认证（M15c GET /tasks 公开浏览）：带令牌则校验（失败 401），
// 无令牌放行——logic 层读不到 userId 即按匿名（userId=0）处理。
func (m *JwtAuthMiddleware) OptionalHandle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			next(w, r)
			return
		}
		resp, err := m.identityRpc.VerifyToken(r.Context(), &identitypb.VerifyTokenReq{Token: token})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, errorx.FromError(err))
			return
		}
		next(w, r.WithContext(WithUserId(r.Context(), resp.UserId)))
	}
}
