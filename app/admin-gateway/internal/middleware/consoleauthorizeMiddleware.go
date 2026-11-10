package middleware

import (
	"bytes"
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/ops/opsClient"
	"gacha_srv/pkg/adminop"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

// consoleAPIPrefix 管理控制台路由前缀（与 .api @server prefix 一致）。
const consoleAPIPrefix = "/console/v2/api"

// ConsoleAuthorizeMiddleware 授权中间件（M14 设计 D4）：目录匹配 → ops-rpc
// Authorize 决策（CEL 条件/数据范围/字段投影/审批模式）→ 拒绝 403 → 通过后
// 字段投影过滤响应 + 异步写审计。内部组合 RequestId + ConsoleAuth 两层
// （goctl 每个 @server 组只保留一个 middleware，这里以组合补齐链路）。
type ConsoleAuthorizeMiddleware struct {
	ops  opsClient.Ops
	auth *ConsoleAuthMiddleware
}

func NewConsoleAuthorizeMiddleware(opsClient opsClient.Ops, auth *ConsoleAuthMiddleware) *ConsoleAuthorizeMiddleware {
	return &ConsoleAuthorizeMiddleware{ops: opsClient, auth: auth}
}

func (m *ConsoleAuthorizeMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. X-Request-Id（与 RequestId 中间件同规则）
		rid := r.Header.Get("X-Request-Id")
		if rid == "" || len(rid) > 128 {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeConsoleRequestIdMissing, "缺少 X-Request-Id 请求头"))
			return
		}

		// 2. 目录匹配（fail-closed 启动校验兜底：漂移按 500 处理）
		op, params, ok := adminop.Match(r.Method, strings.TrimPrefix(r.URL.Path, consoleAPIPrefix))
		if !ok {
			logx.WithContext(r.Context()).Errorf("路由未登记操作目录: %s %s", r.Method, r.URL.Path)
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeServerError, "路由未登记操作目录"))
			return
		}

		// 3. 验签 + 吊销检查（ConsoleAuth 层）
		claims, err := m.auth.verify(r)
		if err != nil {
			restx.Error(w, r, err)
			return
		}

		// 4. ops-rpc Authorize 决策（服务端按 principal_roles 权威解析角色）
		ctx := restx.WithClaims(restx.WithRequestId(r.Context(), rid), claims)
		decision, err := m.ops.Authorize(ctx, &ops.AuthorizeRequest{
			RequestId:   rid,
			ActorId:     claims.ActorID,
			TenantId:    claims.TenantID,
			RoleIds:     claims.Roles, // JWT 快照仅供参考
			Resource:    op.Resource,
			Action:      op.Action,
			OperationId: op.ID,
			Context:     map[string]*ops.AttributeValue{},
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		if !decision.Allowed {
			logx.WithContext(ctx).Infof("授权拒绝: actor=%d op=%s reason=%s",
				claims.ActorID, op.ID, decision.ReasonCode)
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeConsoleForbidden, "无权限执行此操作"))
			return
		}

		// 5. 业务执行 + 字段投影（D4）+ 异步审计
		ctx = restx.WithDecision(ctx, decision)
		ctx = restx.WithOperation(ctx, &op, params)
		fw := &responseFilterWriter{ResponseWriter: w}
		next(fw, r.WithContext(ctx))
		if fw.status == 0 {
			fw.status = http.StatusOK // 未显式 WriteHeader 按 200
		}
		body := fw.buf.Bytes()
		if fw.status == http.StatusOK && len(body) > 0 &&
			(len(decision.AllowedFields) > 0 || len(decision.DeniedFields) > 0) {
			if filtered, ferr := restx.FilterJSON(body, decision.AllowedFields, decision.DeniedFields); ferr == nil {
				body = filtered
			}
		}
		w.WriteHeader(fw.status)
		if len(body) > 0 {
			_, _ = w.Write(body)
		}
		m.writeAudit(ctx, op, params, claims)
	}
}

// responseFilterWriter 缓冲响应体（字段投影用；Header 直通底层）。
type responseFilterWriter struct {
	http.ResponseWriter
	buf    bytes.Buffer
	status int
}

func (f *responseFilterWriter) WriteHeader(code int) {
	if f.status == 0 {
		f.status = code
	}
}

func (f *responseFilterWriter) Write(b []byte) (int, error) {
	if f.status == 0 {
		f.status = http.StatusOK
	}
	return f.buf.Write(b)
}

// writeAudit 异步写审计行（不阻塞响应；失败只记日志）。
func (m *ConsoleAuthorizeMiddleware) writeAudit(ctx context.Context, op adminop.Operation, params map[string]string, claims *restx.ConsoleClaims) {
	auditCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	go func() {
		defer cancel()
		_, err := m.ops.WriteAudit(auditCtx, &ops.WriteAuditRequest{
			TenantId:    claims.TenantID,
			AccountId:   claims.ActorID,
			OperationId: op.ID,
			Resource:    op.Resource,
			Action:      op.Action,
			TargetId:    targetIDFrom(params),
			RequestId:   restx.RequestIdFrom(ctx),
			DetailJson:  "{}",
		})
		if err != nil {
			logx.Errorf("审计写入失败: op=%s request_id=%s err=%v", op.ID, restx.RequestIdFrom(ctx), err)
		}
	}()
}

// targetIDFrom 路径实参拼 target_id（"roleId=3;principalId=7"，无实参为空）。
func targetIDFrom(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, ";")
}
