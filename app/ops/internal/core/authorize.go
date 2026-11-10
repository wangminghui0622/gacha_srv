package core

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"cel.dev/cel-go/cel"

	"gacha_srv/app/ops/ops"
)

// 授权决策语义照抄 server_v2 authorization biz/evaluator.go + effective_grants.go：
// 规则 = 角色权限行（rule_id "rbac.<role_code>.<resource>.<action>"，优先级 1000，effect 恒 ALLOW）；
// 匹配 = operation_id（空=任意）+ resource/action（* 通配）+ 角色（* 通配）+ CEL 条件；
// DENY 规则（effects 表里有则显式拒绝，本仓角色权限无 DENY，保留语义）；
// 多规则：data_scope 取最严（枚举最小）、allowed_fields 取交、denied_fields 取并、approval_mode 取最大。

const (
	// ReasonDefaultDeny 无匹配规则。
	ReasonDefaultDeny = "DEFAULT_DENY"
	// ReasonExplicitDeny 显式拒绝规则命中。
	ReasonExplicitDeny = "EXPLICIT_DENY"
	// ReasonAllow 允许。
	ReasonAllow = "ALLOW"
)

// AuthzRule 授权规则（角色权限行物化）。
type AuthzRule struct {
	RuleID        string
	RoleCodes     []string // 命中此规则的适用角色（含 * 通配）
	Resource      string
	Action        string
	EffectDeny    bool // true = DENY（本仓角色权限恒 false，保留语义）
	Condition     string
	DataScope     ops.DataScope
	AllowedFields []string
	DeniedFields  []string
	ApprovalMode  ops.ApprovalMode
	Priority      int32
	OperationIDs  []string // 空 = 任意操作
}

// AuthzEvaluator 授权决策器（CEL 条件：subject/resource/context 三个 map 变量）。
type AuthzEvaluator struct {
	env *cel.Env
}

// NewAuthzEvaluator 构造决策器。
func NewAuthzEvaluator() (*AuthzEvaluator, error) {
	env, err := cel.NewEnv(
		cel.Variable("subject", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("resource", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("context", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, err
	}
	return &AuthzEvaluator{env: env}, nil
}

// AuthzRequest 授权请求（策略码恒 "console"；roles 服务端权威解析后传入）。
type AuthzRequest struct {
	RequestID          string
	ActorID            uint64
	TenantID           uint64
	RoleCodes          []string
	Resource           string
	Action             string
	OperationID        string
	Subject            map[string]any
	ResourceAttributes map[string]any
	Context            map[string]any
}

// AuthzDecision 授权决策。
type AuthzDecision struct {
	DecisionID     string
	Allowed        bool
	ReasonCode     string
	MatchedRuleIDs []string
	DataScope      ops.DataScope
	AllowedFields  []string
	DeniedFields   []string
	ApprovalMode   ops.ApprovalMode
}

// Authorize 执行授权决策。
func (e *AuthzEvaluator) Authorize(r *AuthzRequest, rules []*AuthzRule) (*AuthzDecision, error) {
	if r == nil || r.RequestID == "" || r.ActorID == 0 || r.TenantID == 0 || r.Resource == "" || r.Action == "" {
		return nil, fmt.Errorf("授权请求无效")
	}
	d := &AuthzDecision{DecisionID: decisionID(r), ReasonCode: ReasonDefaultDeny, ApprovalMode: ops.ApprovalMode_APPROVAL_MODE_NONE}
	roles := stringSet(r.RoleCodes)
	activation := map[string]any{"subject": r.Subject, "resource": r.ResourceAttributes, "context": r.Context}
	sorted := append([]*AuthzRule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })
	var allows []*AuthzRule
	for _, rule := range sorted {
		if !matchesOperation(rule, r.OperationID) || !matchesRule(rule, roles, r.Resource, r.Action) {
			continue
		}
		ok, err := e.evalCondition(rule, activation)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		d.MatchedRuleIDs = append(d.MatchedRuleIDs, rule.RuleID)
		if rule.EffectDeny {
			d.ReasonCode = ReasonExplicitDeny
			return d, nil
		}
		allows = append(allows, rule)
	}
	if len(allows) == 0 {
		return d, nil
	}
	d.Allowed = true
	d.ReasonCode = ReasonAllow
	d.DataScope = restrictiveScope(allows)
	d.AllowedFields = intersectAllowed(allows)
	d.DeniedFields = unionDenied(allows)
	for _, rule := range allows {
		if rule.ApprovalMode > d.ApprovalMode {
			d.ApprovalMode = rule.ApprovalMode
		}
	}
	return d, nil
}

// EffectiveGrants 有效权限（allow 规则按角色过滤，条件规则跳过，通配 DENY 摘除）。
// 返回 grants（resource/action/data_scope/rule_id）与按 resource 分组的 nodes。
func EffectiveGrants(rules []*AuthzRule, roleCodes []string) ([]*ops.PermissionGrant, []*ops.PermissionNode) {
	roles := stringSet(roleCodes)
	var denies, allows []*AuthzRule
	for _, rule := range rules {
		if rule == nil || rule.Condition != "" || len(rule.OperationIDs) != 0 || !ruleMatchesRoles(rule, roles) {
			continue
		}
		if rule.EffectDeny {
			denies = append(denies, rule)
		} else {
			allows = append(allows, rule)
		}
	}
	var grants []*ops.PermissionGrant
	for _, rule := range allows {
		if deniedBy(denies, rule.Resource, rule.Action) {
			continue
		}
		grants = append(grants, &ops.PermissionGrant{Resource: rule.Resource, Action: rule.Action, DataScope: rule.DataScope, RuleId: rule.RuleID})
	}
	sort.SliceStable(grants, func(i, j int) bool {
		if grants[i].Resource == grants[j].Resource {
			return grants[i].Action < grants[j].Action
		}
		return grants[i].Resource < grants[j].Resource
	})
	return grants, groupNodes(grants)
}

func matchesOperation(rule *AuthzRule, operationID string) bool {
	if len(rule.OperationIDs) == 0 {
		return true
	}
	for _, candidate := range rule.OperationIDs {
		if candidate == operationID {
			return true
		}
	}
	return false
}

func matchesRule(rule *AuthzRule, roles map[string]struct{}, resource, action string) bool {
	if rule == nil || (rule.Resource != "*" && rule.Resource != resource) || (rule.Action != "*" && rule.Action != action) {
		return false
	}
	return ruleMatchesRoles(rule, roles)
}

func ruleMatchesRoles(rule *AuthzRule, roles map[string]struct{}) bool {
	if len(rule.RoleCodes) == 0 {
		return true
	}
	for _, role := range rule.RoleCodes {
		if role == "*" {
			return true
		}
		if _, ok := roles[role]; ok {
			return true
		}
	}
	return false
}

func (e *AuthzEvaluator) evalCondition(rule *AuthzRule, activation map[string]any) (bool, error) {
	if rule.RuleID == "" {
		return false, fmt.Errorf("授权规则无效：缺少 rule_id")
	}
	if rule.Condition == "" {
		return true, nil
	}
	ast, issues := e.env.Compile(rule.Condition)
	if issues.Err() != nil {
		return false, fmt.Errorf("编译规则 %s: %w", rule.RuleID, issues.Err())
	}
	program, err := e.env.Program(ast)
	if err != nil {
		return false, err
	}
	out, _, err := program.Eval(activation)
	if err != nil {
		return false, fmt.Errorf("求值规则 %s: %w", rule.RuleID, err)
	}
	ok, valid := out.Value().(bool)
	if !valid {
		return false, fmt.Errorf("规则 %s 必须返回 bool", rule.RuleID)
	}
	return ok, nil
}

func restrictiveScope(rules []*AuthzRule) ops.DataScope {
	scope := ops.DataScope_DATA_SCOPE_UNSPECIFIED
	for _, r := range rules {
		if r.DataScope != ops.DataScope_DATA_SCOPE_UNSPECIFIED && (scope == ops.DataScope_DATA_SCOPE_UNSPECIFIED || r.DataScope < scope) {
			scope = r.DataScope
		}
	}
	return scope
}

func intersectAllowed(rules []*AuthzRule) []string {
	var current map[string]struct{}
	for _, r := range rules {
		if len(r.AllowedFields) == 0 {
			continue
		}
		set := stringSet(r.AllowedFields)
		if current == nil {
			current = set
			continue
		}
		for f := range current {
			if _, ok := set[f]; !ok {
				delete(current, f)
			}
		}
	}
	return sortedKeys(current)
}

func unionDenied(rules []*AuthzRule) []string {
	set := map[string]struct{}{}
	for _, r := range rules {
		for _, f := range r.DeniedFields {
			set[f] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func deniedBy(denies []*AuthzRule, resource, action string) bool {
	for _, rule := range denies {
		if (rule.Resource == "*" || rule.Resource == resource) && (rule.Action == "*" || rule.Action == action) {
			return true
		}
	}
	return false
}

func groupNodes(grants []*ops.PermissionGrant) []*ops.PermissionNode {
	order := []string{}
	byResource := map[string]*ops.PermissionNode{}
	for _, grant := range grants {
		node := byResource[grant.Resource]
		if node == nil {
			node = &ops.PermissionNode{Resource: grant.Resource, DataScope: grant.DataScope}
			byResource[grant.Resource] = node
			order = append(order, grant.Resource)
		}
		node.Actions = append(node.Actions, grant.Action)
		if grant.DataScope != 0 && (node.DataScope == 0 || grant.DataScope < node.DataScope) {
			node.DataScope = grant.DataScope
		}
	}
	nodes := make([]*ops.PermissionNode, 0, len(order))
	for _, resource := range order {
		nodes = append(nodes, byResource[resource])
	}
	return nodes
}

func decisionID(r *AuthzRequest) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s:%s:%s", r.RequestID, r.TenantID, r.Resource, r.Action, r.OperationID)))
	return fmt.Sprintf("authz_%x", sum[:8])
}

func stringSet(in []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, v := range in {
		out[v] = struct{}{}
	}
	return out
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
