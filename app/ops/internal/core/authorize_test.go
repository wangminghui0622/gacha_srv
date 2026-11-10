package core

import (
	"testing"

	"gacha_srv/app/ops/ops"
)

func mustEvaluator(t *testing.T) *AuthzEvaluator {
	t.Helper()
	e, err := NewAuthzEvaluator()
	if err != nil {
		t.Fatalf("NewAuthzEvaluator: %v", err)
	}
	return e
}

func rule(resource, action string, roles []string, scope ops.DataScope) *AuthzRule {
	return &AuthzRule{
		RuleID: "rbac.test." + resource + "." + action, RoleCodes: roles,
		Resource: resource, Action: action, DataScope: scope, Priority: 1000,
	}
}

func request(id string) *AuthzRequest {
	return &AuthzRequest{RequestID: id, ActorID: 1, TenantID: 1, Resource: "gacha.pool", Action: "read"}
}

func TestAuthorize_NoMatchDefaultDeny(t *testing.T) {
	e := mustEvaluator(t)
	d, err := e.Authorize(request("r1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed || d.ReasonCode != ReasonDefaultDeny {
		t.Fatalf("want default deny, got allowed=%v reason=%s", d.Allowed, d.ReasonCode)
	}
}

func TestAuthorize_WildcardRoleAndResource(t *testing.T) {
	e := mustEvaluator(t)
	d, err := e.Authorize(request("r2"), []*AuthzRule{
		rule("*", "*", []string{"*"}, ops.DataScope_DATA_SCOPE_ALL),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed || d.ReasonCode != ReasonAllow {
		t.Fatalf("wildcard should allow, got %+v", d)
	}
	if d.DataScope != ops.DataScope_DATA_SCOPE_ALL {
		t.Fatalf("scope = %v", d.DataScope)
	}
}

func TestAuthorize_DenyBeatsAllow(t *testing.T) {
	e := mustEvaluator(t)
	d, err := e.Authorize(request("r3"), []*AuthzRule{
		rule("*", "*", []string{"*"}, ops.DataScope_DATA_SCOPE_ALL),
		{RuleID: "rbac.deny.all", RoleCodes: []string{"*"}, Resource: "gacha.pool", Action: "read", EffectDeny: true, Priority: 1000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed || d.ReasonCode != ReasonExplicitDeny {
		t.Fatalf("deny must win: %+v", d)
	}
}

func TestAuthorize_RestrictiveScopeAndFieldAggregation(t *testing.T) {
	e := mustEvaluator(t)
	rules := []*AuthzRule{
		{RuleID: "r1", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read",
			DataScope: ops.DataScope_DATA_SCOPE_ALL, AllowedFields: []string{"x", "y"}, DeniedFields: []string{"z"}, Priority: 1000},
		{RuleID: "r2", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read",
			DataScope: ops.DataScope_DATA_SCOPE_TEAM, AllowedFields: []string{"y"}, DeniedFields: []string{"w"}, Priority: 1000},
	}
	req := request("r4")
	req.RoleCodes = []string{"a"}
	d, err := e.Authorize(req, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed || d.DataScope != ops.DataScope_DATA_SCOPE_TEAM {
		t.Fatalf("most restrictive scope wins: %+v", d)
	}
	if len(d.AllowedFields) != 1 || d.AllowedFields[0] != "y" {
		t.Fatalf("allowed fields should intersect to [y], got %v", d.AllowedFields)
	}
	if len(d.DeniedFields) != 2 || d.DeniedFields[0] != "w" || d.DeniedFields[1] != "z" {
		t.Fatalf("denied fields should union to [w z], got %v", d.DeniedFields)
	}
}

func TestAuthorize_ApprovalModeTakesMax(t *testing.T) {
	e := mustEvaluator(t)
	req := request("r5")
	req.RoleCodes = []string{"a"}
	d, err := e.Authorize(req, []*AuthzRule{
		{RuleID: "r1", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read", Priority: 1000, ApprovalMode: ops.ApprovalMode_APPROVAL_MODE_SINGLE},
		{RuleID: "r2", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read", Priority: 1000, ApprovalMode: ops.ApprovalMode_APPROVAL_MODE_DUAL_CONTROL},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.ApprovalMode != ops.ApprovalMode_APPROVAL_MODE_DUAL_CONTROL {
		t.Fatalf("approval mode should be max (DUAL_CONTROL), got %v", d.ApprovalMode)
	}
}

func TestAuthorize_CELCondition(t *testing.T) {
	e := mustEvaluator(t)
	req := request("r6")
	req.RoleCodes = []string{"a"}
	req.Subject = map[string]any{"vip": true}

	// 条件满足 → 放行
	d, err := e.Authorize(req, []*AuthzRule{
		{RuleID: "r1", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read", Priority: 1000,
			Condition: `subject.vip == true`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed {
		t.Fatalf("condition matched but denied: %+v", d)
	}
	// 条件不满足 → 无匹配 → 默认拒绝
	req.Subject = map[string]any{"vip": false}
	d, err = e.Authorize(req, []*AuthzRule{
		{RuleID: "r1", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read", Priority: 1000,
			Condition: `subject.vip == true`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Fatal("condition unmet should fall through to default deny")
	}
	// 条件编译错误 → 报错
	_, err = e.Authorize(req, []*AuthzRule{
		{RuleID: "r1", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read", Priority: 1000,
			Condition: `subject.vip =`},
	})
	if err == nil {
		t.Fatal("invalid CEL should return error")
	}
}

func TestAuthorize_OperationIDRestriction(t *testing.T) {
	e := mustEvaluator(t)
	req := request("r7")
	req.OperationID = "console.pool.get"
	req.RoleCodes = []string{"a"}
	rules := []*AuthzRule{
		{RuleID: "r1", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read", Priority: 1000,
			OperationIDs: []string{"console.pool.list"}},
	}
	d, err := e.Authorize(req, rules)
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Fatal("operation id mismatch should deny")
	}
	req.OperationID = "console.pool.list"
	d, err = e.Authorize(req, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed {
		t.Fatal("operation id match should allow")
	}
}

func TestEffectiveGrants_SkipsConditionalAndGroupsNodes(t *testing.T) {
	rules := []*AuthzRule{
		{RuleID: "r1", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "read", DataScope: ops.DataScope_DATA_SCOPE_ALL, Priority: 1000},
		{RuleID: "r2", RoleCodes: []string{"a"}, Resource: "gacha.pool", Action: "write", DataScope: ops.DataScope_DATA_SCOPE_TEAM, Priority: 1000},
		{RuleID: "r3", RoleCodes: []string{"a"}, Resource: "catalog.product", Action: "read", DataScope: ops.DataScope_DATA_SCOPE_ALL, Priority: 1000,
			Condition: "subject.vip == true"},
	}
	grants, nodes := EffectiveGrants(rules, []string{"a"})
	if len(grants) != 2 {
		t.Fatalf("conditional rule should be skipped, grants = %+v", grants)
	}
	if len(nodes) != 1 || nodes[0].Resource != "gacha.pool" || len(nodes[0].Actions) != 2 {
		t.Fatalf("grouping wrong: %+v", nodes)
	}
	if nodes[0].DataScope != ops.DataScope_DATA_SCOPE_TEAM {
		t.Fatalf("node scope should be most restrictive, got %v", nodes[0].DataScope)
	}
}
