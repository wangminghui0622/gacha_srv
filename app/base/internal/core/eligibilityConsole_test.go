package core

import (
	"bytes"
	"testing"

	"gacha_srv/app/base/base"
)

func TestConfigHashOf(t *testing.T) {
	basev := &base.PolicyVersion{
		PolicyId:    5,
		Version:     2,
		Status:      base.PolicyStatus_POLICY_STATUS_DRAFT,
		Scene:       base.EligibilityScene_ELIGIBILITY_SCENE_DRAW,
		Combination: base.RuleCombination_RULE_COMBINATION_ALL,
		Rules: []*base.EligibilityRule{
			{RuleId: "r1", Expression: "facts.user_id == 3", FailureCode: "USER_BLOCKED"},
		},
		RequiredFacts: []string{"user_id", "box_id"},
		ConfigHash:    []byte{1, 2, 3},
	}
	h1 := ConfigHashOf(basev)
	if len(h1) != 32 {
		t.Fatalf("config_hash 应为 32 字节: %d", len(h1))
	}
	// 幂等
	h2 := ConfigHashOf(basev)
	if !bytes.Equal(h1, h2) {
		t.Error("同内容哈希不稳定")
	}
	// PolicyId/Version/Status/ConfigHash 置零不影响哈希
	alt := &base.PolicyVersion{
		PolicyId: 9, Version: 7, Status: base.PolicyStatus_POLICY_STATUS_PUBLISHED,
		Scene:       base.EligibilityScene_ELIGIBILITY_SCENE_DRAW,
		Combination: base.RuleCombination_RULE_COMBINATION_ALL,
		Rules: []*base.EligibilityRule{
			{RuleId: "r1", Expression: "facts.user_id == 3", FailureCode: "USER_BLOCKED"},
		},
		RequiredFacts: []string{"user_id", "box_id"},
		ConfigHash:    []byte{9, 9, 9},
	}
	if !bytes.Equal(h1, ConfigHashOf(alt)) {
		t.Error("被置零字段差异不应影响哈希")
	}
	// required_facts 排序不敏感
	alt.RequiredFacts = []string{"box_id", "user_id"}
	if !bytes.Equal(h1, ConfigHashOf(alt)) {
		t.Error("requiredFacts 顺序不应影响哈希")
	}
	// 内容差异必须变哈希
	alt.Rules[0].FailureCode = "OTHER"
	if bytes.Equal(h1, ConfigHashOf(alt)) {
		t.Error("failureCode 差异应改变哈希")
	}
}

func TestSceneIntOf(t *testing.T) {
	for s, want := range map[string]int32{"DISCOVER": 1, "VIEW": 2, "DRAW": 3, "BOOK": 4} {
		got, ok := SceneIntOf(s)
		if !ok || got != want {
			t.Errorf("SceneIntOf(%s)=%d,%v 错", s, got, ok)
		}
	}
	if _, ok := SceneIntOf("GASHAPON"); ok {
		t.Error("未知场景应 false")
	}
}

func TestValidateRuleExpression(t *testing.T) {
	if err := ValidateRuleExpression("facts.user_id == 3"); err != nil {
		t.Errorf("合法 bool 表达式被拒: %v", err)
	}
	if err := ValidateRuleExpression("facts.box_id != 2 && facts.user_id > 0"); err != nil {
		t.Errorf("复合表达式被拒: %v", err)
	}
	if err := ValidateRuleExpression("facts.user_id =="); err == nil {
		t.Error("语法错误应报错")
	}
	if err := ValidateRuleExpression("1 + 1"); err == nil {
		t.Error("非 bool 输出应报错")
	}
	if err := ValidateRuleExpression("unknown_var"); err == nil {
		t.Error("未声明变量应报错")
	}
}

func TestEvaluateRuleExpression(t *testing.T) {
	facts := map[string]any{"user_id": int64(3), "box_id": int64(2)}
	ok, err := EvaluateRuleExpression("facts.user_id == 3", facts)
	if err != nil || !ok {
		t.Errorf("user_id==3 应通过: %v %v", ok, err)
	}
	ok, err = EvaluateRuleExpression("facts.box_id != 2", facts)
	if err != nil || ok {
		t.Errorf("box_id!=2 应拒绝: %v %v", ok, err)
	}
	if _, err := EvaluateRuleExpression("facts.missing == 1", facts); err == nil {
		t.Error("缺键求值应报错（调用方保守拒绝）")
	}
}

func TestValidatePolicyDraft(t *testing.T) {
	good := func() (uint64, string, string, base.EligibilityScene, base.RuleCombination,
		[]*base.EligibilityRule, []string) {
		return 1, "p.draw.deny", "测试政策", base.EligibilityScene_ELIGIBILITY_SCENE_DRAW,
			base.RuleCombination_RULE_COMBINATION_ALL,
			[]*base.EligibilityRule{{RuleId: "r1", Expression: "facts.user_id == 3", FailureCode: "USER_BLOCKED"}},
			[]string{"user_id"}
	}
	tID, code, name, scene, comb, rules, facts := good()
	if err := ValidatePolicyDraft(tID, code, name, scene, comb, rules, facts, true); err != nil {
		t.Fatalf("合法草稿被拒: %v", err)
	}	// 修订模式 code 可空
	if err := ValidatePolicyDraft(1, "", "测试政策", base.EligibilityScene_ELIGIBILITY_SCENE_DRAW,
		base.RuleCombination_RULE_COMBINATION_ANY,
		[]*base.EligibilityRule{{RuleId: "r1", Expression: "facts.user_id > 0", FailureCode: "X"}},
		[]string{"user_id"}, false); err != nil {
		t.Fatalf("修订模式合法草稿被拒: %v", err)
	}
	cases := []struct {
		name string
		mut  func(*uint64, *string, *string, *base.EligibilityScene, *base.RuleCombination,
			*[]*base.EligibilityRule, *[]string)
	}{
		{"tenant 非 1", func(t *uint64, _ *string, _ *string, _ *base.EligibilityScene, _ *base.RuleCombination,
			_ *[]*base.EligibilityRule, _ *[]string) { *t = 0 }},
		{"code 非法", func(_ *uint64, c *string, _ *string, _ *base.EligibilityScene, _ *base.RuleCombination,
			_ *[]*base.EligibilityRule, _ *[]string) { *c = "Bad Code" }},
		{"name 空", func(_ *uint64, _ *string, n *string, _ *base.EligibilityScene, _ *base.RuleCombination,
			_ *[]*base.EligibilityRule, _ *[]string) { *n = "" }},
		{"scene 非法", func(_ *uint64, _ *string, _ *string, s *base.EligibilityScene, _ *base.RuleCombination,
			_ *[]*base.EligibilityRule, _ *[]string) { *s = base.EligibilityScene_ELIGIBILITY_SCENE_UNSPECIFIED }},
		{"combination 非法", func(_ *uint64, _ *string, _ *string, _ *base.EligibilityScene, c *base.RuleCombination,
			_ *[]*base.EligibilityRule, _ *[]string) { *c = base.RuleCombination_RULE_COMBINATION_UNSPECIFIED }},
		{"规则 ID 重复", func(_ *uint64, _ *string, _ *string, _ *base.EligibilityScene, _ *base.RuleCombination,
			rs *[]*base.EligibilityRule, _ *[]string) {
			*rs = append(*rs, &base.EligibilityRule{RuleId: "r1", Expression: "facts.user_id > 0", FailureCode: "Y"})
		}},
		{"failureCode 空", func(_ *uint64, _ *string, _ *string, _ *base.EligibilityScene, _ *base.RuleCombination,
			rs *[]*base.EligibilityRule, _ *[]string) { (*rs)[0].FailureCode = "" }},
		{"requiredFacts 空键", func(_ *uint64, _ *string, _ *string, _ *base.EligibilityScene, _ *base.RuleCombination,
			_ *[]*base.EligibilityRule, fs *[]string) { *fs = append(*fs, "") }},
		{"rules 空", func(_ *uint64, _ *string, _ *string, _ *base.EligibilityScene, _ *base.RuleCombination,
			rs *[]*base.EligibilityRule, _ *[]string) { *rs = nil }},
	}
	for _, c := range cases {
		tID, code, name, scene, comb, rules, facts := good()
		c.mut(&tID, &code, &name, &scene, &comb, &rules, &facts)
		if err := ValidatePolicyDraft(tID, code, name, scene, comb, rules, facts, true); err == nil {
			t.Errorf("%s: 应报错", c.name)
		}
	}
}

func TestSeverityScoreOf(t *testing.T) {
	if SeverityScoreOf("LOW") != 30 || SeverityScoreOf("MEDIUM") != 60 || SeverityScoreOf("HIGH") != 90 {
		t.Error("severity 映射错")
	}
	if SeverityScoreOf("UNKNOWN") != 0 {
		t.Error("未知 severity 应为 0")
	}
}
