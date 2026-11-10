// Package restx 管理网关响应辅助（M14 设计 D9，与播放器网关信封不同）：
// 成功/失败响应均为 protojson 编码——camelCase 字段名、uint64 序列化为
// JSON 字符串、枚举为名字面量（如 "DATA_SCOPE_ALL"），与 server_v2 kratos
// http 输出逐字一致；错误体为 kratos 形 {"code":<HTTP状态码>,"message":"...","reason":"..."}，
// 且 HTTP 状态码与 code 一致（不是播放器网关的恒 200 {code,msg,data} 信封）。
package restx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/adminop"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// 上下文键（请求 ID / JWT claims / 授权决策 / 目录操作）。
type ctxKey int

const (
	ctxRequestId ctxKey = iota
	ctxClaims
	ctxDecision
	ctxOperation
	ctxOpParams
)

// ConsoleClaims 控制台 JWT claims（网关本地验签后放入上下文）。
type ConsoleClaims struct {
	ActorID   uint64
	TenantID  uint64
	Name      string
	Roles     []string
	SessionID string // JWT jti
}

// WithRequestId / RequestIdFrom 请求 ID 存取。
func WithRequestId(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxRequestId, id)
}

func RequestIdFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxRequestId).(string)
	return id
}

// WithClaims / ClaimsFrom JWT claims 存取。
func WithClaims(ctx context.Context, c *ConsoleClaims) context.Context {
	return context.WithValue(ctx, ctxClaims, c)
}

func ClaimsFrom(ctx context.Context) *ConsoleClaims {
	c, _ := ctx.Value(ctxClaims).(*ConsoleClaims)
	return c
}

// WithDecision / DecisionFrom 授权决策存取（ConsoleAuthorize 写入）。
func WithDecision(ctx context.Context, d *ops.AuthorizationDecision) context.Context {
	return context.WithValue(ctx, ctxDecision, d)
}

func DecisionFrom(ctx context.Context) *ops.AuthorizationDecision {
	d, _ := ctx.Value(ctxDecision).(*ops.AuthorizationDecision)
	return d
}

// WithOperation / OperationFrom 目录操作存取（含路径实参）。
func WithOperation(ctx context.Context, op *adminop.Operation, params map[string]string) context.Context {
	ctx = context.WithValue(ctx, ctxOperation, op)
	return context.WithValue(ctx, ctxOpParams, params)
}

func OperationFrom(ctx context.Context) (*adminop.Operation, map[string]string) {
	op, _ := ctx.Value(ctxOperation).(*adminop.Operation)
	params, _ := ctx.Value(ctxOpParams).(map[string]string)
	return op, params
}

// ── 请求解析 ──

var protoUnmarshal = protojson.UnmarshalOptions{}

// ParseProto 从请求体解析 protojson（接受 camelCase 与 snake_case，与 server_v2
// kratos http 一致；空体按空消息处理）。
func ParseProto(r *http.Request, m proto.Message) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return errorx.NewCodeError(errorx.CodeParamError, "读取请求体失败")
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		body = []byte("{}")
	}
	if err = protoUnmarshal.Unmarshal(body, m); err != nil {
		return errorx.NewCodeError(errorx.CodeParamError, "请求体不是合法的 JSON："+err.Error())
	}
	return nil
}

// PathValue 读取路径参数（go-zero pathvar）。
func PathValue(r *http.Request, name string) string {
	return pathvar.Vars(r)[name]
}

// PathUint64 读取 uint64 路径参数；缺失/非法返回 error。
func PathUint64(r *http.Request, name string) (uint64, error) {
	raw := PathValue(r, name)
	if raw == "" {
		return 0, errorx.NewCodeError(errorx.CodeParamError, fmt.Sprintf("路径参数 %s 缺失", name))
	}
	var v uint64
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil || v == 0 {
		return 0, errorx.NewCodeError(errorx.CodeParamError, fmt.Sprintf("路径参数 %s 非法：%s", name, raw))
	}
	return v, nil
}

// PathUint64AllowZero 同 PathUint64 但允许 0（任务计划 :version=0 表示最新，server_v2 同款）。
func PathUint64AllowZero(r *http.Request, name string) (uint64, error) {
	raw := PathValue(r, name)
	if raw == "" {
		return 0, errorx.NewCodeError(errorx.CodeParamError, fmt.Sprintf("路径参数 %s 缺失", name))
	}
	var v uint64
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil {
		return 0, errorx.NewCodeError(errorx.CodeParamError, fmt.Sprintf("路径参数 %s 非法：%s", name, raw))
	}
	return v, nil
}

// QueryUint64 读取可选 uint64 查询参数（缺失返回 0；非法返回 error）。
func QueryUint64(r *http.Request, name string) (uint64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	var v uint64
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil {
		return 0, errorx.NewCodeError(errorx.CodeParamError, fmt.Sprintf("查询参数 %s 非法：%s", name, raw))
	}
	return v, nil
}

// QueryUint32 同 QueryUint64 的 uint32 版。
func QueryUint32(r *http.Request, name string) (uint32, error) {
	v, err := QueryUint64(r, name)
	return uint32(v), err
}

// QueryString 读取可选字符串查询参数（缺失返回 ""）。
func QueryString(r *http.Request, name string) string {
	return r.URL.Query().Get(name)
}

// QueryEnum 读取可选枚举查询参数（proto 名字面量，如 "TASK_PERIOD_DAILY"）；
// 缺失返回 0，未知名字返回 error（与 server_v2 kratos 枚举绑定一致）。
func QueryEnum(r *http.Request, name string, values map[string]int32) (int32, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	v, ok := values[raw]
	if !ok {
		return 0, errorx.NewCodeError(errorx.CodeParamError, fmt.Sprintf("查询参数 %s 非法：%s", name, raw))
	}
	return v, nil
}

// QueryTimestamp 读取可选时间查询参数（RFC3339，与 kratos
// google.protobuf.Timestamp 查询绑定一致）；缺失返回 nil。
func QueryTimestamp(r *http.Request, name string) (*timestamppb.Timestamp, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeParamError, fmt.Sprintf("查询参数 %s 非法（须 RFC3339）：%s", name, raw))
	}
	return timestamppb.New(t), nil
}

// ── 响应写出 ──

var protoMarshal = protojson.MarshalOptions{UseProtoNames: false}

// WriteProto 写 protojson 成功响应（camelCase、uint64 → 字符串）。
func WriteProto(w http.ResponseWriter, r *http.Request, m proto.Message) {
	body, err := protoMarshal.Marshal(m)
	if err != nil {
		Error(w, r, err)
		return
	}
	// protojson ≥v1.36 单行输出按二进制哈希随机在逗号后插空格（官方"不稳定输出"约定），
	// 每轮编译都会漂移——compact 归一，保证 wire 输出逐字稳定（照抄 server_v2 kratos）。
	var buf bytes.Buffer
	if err := json.Compact(&buf, body); err != nil {
		Error(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, buf.Bytes())
}

// WriteJSON 写普通 JSON 成功响应（健康检查等非 proto 端点）。
func WriteJSON(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		Error(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, body)
}

func writeJSON(w http.ResponseWriter, r *http.Request, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if rid := RequestIdFrom(r.Context()); rid != "" {
		w.Header().Set("X-Request-Id", rid)
	}
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

// ── 错误映射（kratos 形错误体，D9）──

// kratosError kratos 风格错误体（HTTP 状态码 = code，reason 为稳定机器码）。
type kratosError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

// MapError 错误 → (HTTP 状态码, reason 机器码)。
func MapError(err error) (int, string) {
	// 业务错误（errorx 码，跨服务由 ClientErrorInterceptor 还原）
	var ce *errorx.CodeError
	if errors.As(err, &ce) {
		return mapCodeError(ce)
	}
	// gRPC 传输层错误（zrpc 未还原的 status）
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable:
			return http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"
		case codes.DeadlineExceeded, codes.Canceled:
			return http.StatusGatewayTimeout, "GATEWAY_TIMEOUT"
		default:
			return http.StatusInternalServerError, "INTERNAL_ERROR"
		}
	}
	return http.StatusInternalServerError, "INTERNAL_ERROR"
}

func mapCodeError(ce *errorx.CodeError) (int, string) {
	switch ce.Code {
	case errorx.CodeParamError:
		return http.StatusBadRequest, "VALIDATION_ERROR"
	case errorx.CodeConsoleAccountNotFound:
		return http.StatusNotFound, "CONSOLE_ACCOUNT_NOT_FOUND"
	case errorx.CodeConsoleForbidden:
		return http.StatusForbidden, "CONSOLE_FORBIDDEN"
	case errorx.CodeConsoleTokenInvalid:
		return http.StatusUnauthorized, "CONSOLE_TOKEN_INVALID"
	case errorx.CodeConsoleRoleNotFound:
		return http.StatusNotFound, "CONSOLE_ROLE_NOT_FOUND"
	case errorx.CodeConsoleRoleCodeExists:
		return http.StatusConflict, "CONSOLE_ROLE_CODE_EXISTS"
	case errorx.CodeConsoleAccountDisabled:
		return http.StatusForbidden, "CONSOLE_ACCOUNT_DISABLED"
	case errorx.CodeConsoleRequestIdMissing:
		return http.StatusBadRequest, "CONSOLE_REQUEST_ID_MISSING"
	case errorx.CodeConsoleLoginInvalid:
		return http.StatusUnauthorized, "CONSOLE_LOGIN_INVALID"
	case errorx.CodeConsoleTotpInvalid:
		return http.StatusUnauthorized, "CONSOLE_TOTP_INVALID"
	case errorx.CodeConsoleLoginTicketInvalid:
		return http.StatusUnauthorized, "CONSOLE_LOGIN_TICKET_INVALID"
	case errorx.CodeConsoleRoleVersionConflict:
		return http.StatusConflict, "CONSOLE_VERSION_CONFLICT"
	case errorx.CodeConsoleUnknownPermission:
		return http.StatusBadRequest, "CONSOLE_UNKNOWN_PERMISSION"
	case errorx.CodeConsoleAccountExists:
		return http.StatusConflict, "CONSOLE_ACCOUNT_EXISTS"
	case errorx.CodeConsoleRefreshInvalid:
		return http.StatusUnauthorized, "CONSOLE_REFRESH_INVALID"
	case errorx.CodeConsoleIdempotencyRequired:
		return http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED"
	// ── M14b 赏池控制台（draw 域 2006-2010）──
	case errorx.CodePoolDraftInvalid:
		return http.StatusBadRequest, "POOL_DRAFT_INVALID"
	case errorx.CodePoolVersionNotFound:
		return http.StatusNotFound, "POOL_VERSION_NOT_FOUND"
	case errorx.CodePoolVersionConflict:
		return http.StatusConflict, "POOL_VERSION_CONFLICT"
	case errorx.CodePoolPublicationConflict:
		return http.StatusConflict, "POOL_PUBLICATION_CONFLICT"
	case errorx.CodePoolDraftKeyConflict:
		return http.StatusConflict, "POOL_DRAFT_REQUEST_CONFLICT"
	// game 域透传（assign-lucky 经 pool 控制台暴露，3027）
	case errorx.CodeLuckyAssignInvalid:
		return http.StatusBadRequest, "LUCKY_ASSIGN_INVALID"
	// ── M14c 商品目录控制台（商城域 4001-4003）──
	case errorx.CodeCatalogNotFound:
		return http.StatusNotFound, "PRODUCT_NOT_FOUND"
	case errorx.CodeCatalogConflict:
		return http.StatusConflict, "PRODUCT_CONFLICT"
	case errorx.CodeCatalogInvalid:
		return http.StatusBadRequest, "PRODUCT_INVALID"
	// ── M14d 活动/券控制台（game 域 3000 段）──
	case errorx.CodeCampaignNotFound:
		return http.StatusNotFound, "CAMPAIGN_NOT_FOUND"
	case errorx.CodeCampaignConflict:
		return http.StatusConflict, "CAMPAIGN_CONFLICT"
	case errorx.CodeCampaignParamInvalid:
		return http.StatusBadRequest, "CAMPAIGN_INVALID"
	case errorx.CodeCouponTemplateNotFound:
		return http.StatusNotFound, "COUPON_TEMPLATE_NOT_FOUND"
	case errorx.CodeCouponTemplateConflict:
		return http.StatusConflict, "COUPON_TEMPLATE_CONFLICT"
	case errorx.CodeCouponInvalid:
		return http.StatusBadRequest, "COUPON_INVALID"
	// ── M14d 条件政策/风控控制台（base 域 5000 段）──
	case errorx.CodePolicyNotFound:
		return http.StatusNotFound, "POLICY_NOT_FOUND"
	case errorx.CodePolicyConflict:
		return http.StatusConflict, "POLICY_CONFLICT"
	case errorx.CodePolicyInvalid:
		return http.StatusBadRequest, "POLICY_INVALID"
	case errorx.CodeRiskCaseNotFound:
		return http.StatusNotFound, "RISK_CASE_NOT_FOUND"
	case errorx.CodeRiskCaseStateInvalid:
		return http.StatusConflict, "RISK_CASE_CONFLICT"
	case errorx.CodeRiskCaseInvalid:
		return http.StatusBadRequest, "RISK_CASE_INVALID"
	// ── M14d 支付路由控制台（payment 域 8000 段）──
	case errorx.CodePaymentRouteNotFound:
		return http.StatusNotFound, "PAYMENT_ROUTE_NOT_FOUND"
	case errorx.CodePaymentRouteVersionConflict:
		return http.StatusConflict, "PAYMENT_ROUTE_VERSION_CONFLICT"
	case errorx.CodePaymentRouteInvalid:
		return http.StatusBadRequest, "PAYMENT_ROUTE_INVALID"
	// ── M14e 任务计划/报表控制台（ops 域 7000 段）──
	case errorx.CodeTaskPlanNotFound:
		return http.StatusNotFound, "TASK_PLAN_NOT_FOUND"
	case errorx.CodeTaskPlanConflict:
		return http.StatusConflict, "TASK_PLAN_CONFLICT"
	case errorx.CodeTaskPlanInvalid:
		return http.StatusBadRequest, "TASK_PLAN_INVALID"
	case errorx.CodeTaskPlanNotReady:
		return http.StatusBadRequest, "TASK_PLAN_NOT_READY"
	case errorx.CodeDashboardNotFound:
		return http.StatusNotFound, "DASHBOARD_NOT_FOUND"
	case errorx.CodeDashboardConflict:
		return http.StatusConflict, "DASHBOARD_CONFLICT"
	case errorx.CodeDashboardInvalid:
		return http.StatusBadRequest, "DASHBOARD_INVALID"
	case errorx.CodeDashboardNotPublished:
		return http.StatusBadRequest, "DASHBOARD_NOT_PUBLISHED"
	// M14e 网关守卫（server_v2 kratos reason 同款）
	case errorx.CodeConsoleTenantMismatch:
		return http.StatusForbidden, "TENANT_MISMATCH"
	case errorx.CodeConsoleDataScopeDenied:
		return http.StatusForbidden, "DATA_SCOPE_DENIED"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

// Error 写 kratos 形错误体；内部错误详情只进日志不进响应体。
func Error(w http.ResponseWriter, r *http.Request, err error) {
	statusCode, reason := MapError(err)
	message := err.Error()
	if statusCode >= http.StatusInternalServerError {
		logx.WithContext(r.Context()).Errorf("管理网关内部错误: %v", err)
		message = "内部错误"
	}
	writeJSON(w, r, statusCode, mustJSON(kratosError{Code: statusCode, Message: message, Reason: reason}))
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"code":500,"message":"内部错误","reason":"INTERNAL_ERROR"}`)
	}
	return b
}

// ── 字段投影（D4：decision.allowed_fields/denied_fields 过滤响应 JSON）──

// FilterJSON 按 allowed/denied 点分路径过滤 JSON（均空时原样返回）。
// denied 优先：命中或被其子树覆盖的键删除；allowed 非空时未命中的键删除。
func FilterJSON(data []byte, allowed, denied []string) ([]byte, error) {
	if len(allowed) == 0 && len(denied) == 0 {
		return data, nil
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return data, err // 非 JSON 不投影
	}
	project(&v, "", allowed, denied)
	return json.Marshal(v)
}

func project(node *any, prefix string, allowed, denied []string) {
	switch t := (*node).(type) {
	case map[string]any:
		for key, child := range t {
			full := joinPath(prefix, key)
			if matchesAny(full, denied) {
				delete(t, key)
				continue
			}
			if childMap, ok := child.(map[string]any); ok {
				var nested any = childMap
				project(&nested, full, allowed, denied)
				if len(allowed) > 0 && !keepsAny(full, allowed) {
					delete(t, key)
				}
				continue
			}
			if len(allowed) > 0 && !contains(full, allowed) {
				delete(t, key)
			}
		}
	case []any:
		for i := range t {
			project(&t[i], prefix, allowed, denied)
		}
	}
}

// matchesAny 路径命中 denied（自身或被 denied 子树覆盖）。
func matchesAny(path string, denied []string) bool {
	for _, d := range denied {
		if path == d || strings.HasPrefix(path, d+".") {
			return true
		}
	}
	return false
}

// keepsAny 子树下是否还有 allowed 路径（决定父键保留）。
func keepsAny(path string, allowed []string) bool {
	for _, a := range allowed {
		if a == path || strings.HasPrefix(a, path+".") {
			return true
		}
	}
	return false
}

func contains(path string, list []string) bool {
	for _, item := range list {
		if item == path {
			return true
		}
	}
	return false
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// ValidateProto 校验 proto 请求体的基础字段：枚举字段不得为 UNSPECIFIED（0）。
// 业务必填/范围/冲突校验在 RPC 服务端（权威），网关层只拦截「未指定枚举」这种格式错误；
// 允许 UNSPECIFIED 的枚举字段（如查询可选的 channel/scene）通过 skipEnum 白名单跳过（传字段名）。
// 嵌套消息递归校验。
func ValidateProto(m proto.Message, skipEnum ...string) error {
	skip := make(map[string]struct{}, len(skipEnum))
	for _, name := range skipEnum {
		skip[name] = struct{}{}
	}
	var bad []string
	validateEnumRecursive(m.ProtoReflect(), skip, &bad)
	if len(bad) == 0 {
		return nil
	}
	return errorx.NewCodeError(errorx.CodeParamError,
		fmt.Sprintf("参数 %s 不能为未指定值", strings.Join(bad, "、")))
}

// validateEnumRecursive 递归遍历 proto 消息：枚举字段为 UNSPECIFIED(0) 且不在 skip 白名单时记录。
func validateEnumRecursive(m protoreflect.Message, skip map[string]struct{}, bad *[]string) {
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.Kind() == protoreflect.EnumKind:
			if _, ok := skip[string(fd.Name())]; ok {
				return true
			}
			if fd.IsList() {
				list := v.List()
				for i := 0; i < list.Len(); i++ {
					if list.Get(i).Enum() == 0 {
						*bad = append(*bad, string(fd.Name()))
						break
					}
				}
			} else if v.Enum() == 0 {
				*bad = append(*bad, string(fd.Name()))
			}
		case fd.Kind() == protoreflect.MessageKind:
			if fd.IsList() {
				list := v.List()
				for i := 0; i < list.Len(); i++ {
					validateEnumRecursive(list.Get(i).Message(), skip, bad)
				}
			} else if v.Message().IsValid() {
				validateEnumRecursive(v.Message(), skip, bad)
			}
		}
		return true
	})
}
