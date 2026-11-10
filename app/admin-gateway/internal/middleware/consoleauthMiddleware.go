package middleware

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/pkg/utils/errorx"

	"github.com/golang-jwt/jwt/v4"
	"github.com/redis/go-redis/v9"
)

// ConsoleAuthMiddleware 会话中间件（M14 设计 D3）：Ed25519 公钥本地验签 JWT
// （iss/aud 校验）→ Redis 吊销键检查 → claims 入上下文。内部先跑 RequestId 层
// （goctl 每个 @server 组只保留一个 middleware，这里以组合补齐链路）。
type ConsoleAuthMiddleware struct {
	publicKey ed25519.PublicKey
	issuer    string
	audience  string
	rdb       *redis.Client
	requestId *RequestIdMiddleware
}

func NewConsoleAuthMiddleware(publicKey ed25519.PublicKey, issuer, audience string, rdb *redis.Client) *ConsoleAuthMiddleware {
	return &ConsoleAuthMiddleware{
		publicKey: publicKey, issuer: issuer, audience: audience, rdb: rdb,
		requestId: NewRequestIdMiddleware(),
	}
}

func (m *ConsoleAuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	withRequestId := m.requestId.Handle(next)
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := m.verify(r)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		withRequestId(w, r.WithContext(restx.WithClaims(r.Context(), claims)))
	}
}

// verify 验签 + 吊销检查，返回 claims。
func (m *ConsoleAuthMiddleware) verify(r *http.Request) (*restx.ConsoleClaims, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少 Bearer 令牌")
	}
	parsed, err := jwt.Parse(strings.TrimPrefix(header, "Bearer "), func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "签名算法不受支持")
		}
		return m.publicKey, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}))
	if err != nil || !parsed.Valid {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "令牌无效或已过期")
	}
	mapClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "令牌声明格式非法")
	}
	if mapClaims["iss"] != m.issuer || mapClaims["aud"] != m.audience {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "令牌签发方或受众不符")
	}
	claims := &restx.ConsoleClaims{
		ActorID:   uint64FromClaims(mapClaims["actor_id"]),
		TenantID:  uint64FromClaims(mapClaims["tenant_id"]),
		Name:      stringFromClaims(mapClaims["name"]),
		Roles:     rolesFromClaims(mapClaims["roles"]),
		SessionID: stringFromClaims(mapClaims["jti"]),
	}
	if claims.ActorID == 0 || claims.TenantID == 0 || claims.SessionID == "" {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "令牌声明缺少主体信息")
	}
	// 吊销检查（fail-closed：Redis 不可用宁可 503 也不放行已吊销会话）
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	revoked, err := m.rdb.Exists(ctx, "console:session:revoked:"+claims.SessionID).Result()
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "会话吊销检查不可用")
	}
	if revoked > 0 {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "会话已吊销")
	}
	return claims, nil
}

func uint64FromClaims(v any) uint64 {
	switch t := v.(type) {
	case float64:
		return uint64(t)
	case int64:
		return uint64(t)
	case string:
		var out uint64
		_, _ = fmt.Sscanf(t, "%d", &out)
		return out
	}
	return 0
}

func stringFromClaims(v any) string {
	s, _ := v.(string)
	return s
}

func rolesFromClaims(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
