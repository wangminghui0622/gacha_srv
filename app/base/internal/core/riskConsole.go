// Package core M14d 控制台纯函数（base 域）：政策草稿校验 + 风控 severity 映射。
package core

import (
	"fmt"
	"strings"

	"gacha_srv/app/base/base"
)

// ValidatePolicyDraft 政策草稿/修订校验（D3 + 5008 CodePolicyInvalid）：
// tenant=1、code 正则、name 1..128、scene/combination 枚举、规则与必需事实合法。
// isRevision=true 时 code 为空（沿用头编码）且 requiredFacts/规则照常校验。
func ValidatePolicyDraft(tenantID uint64, code, name string, scene base.EligibilityScene,
	combination base.RuleCombination, rules []*base.EligibilityRule, requiredFacts []string,
	requireCode bool) error {
	if tenantID != 1 {
		return fmt.Errorf("tenantId 须为 1")
	}
	if requireCode {
		if !policyCodeRe.MatchString(code) {
			return fmt.Errorf("policyCode 非法（须匹配 ^[a-z][a-z0-9_.-]{1,63}$）")
		}
	}
	if name == "" || len(name) > 128 {
		return fmt.Errorf("name 为空或超长")
	}
	if scene < base.EligibilityScene_ELIGIBILITY_SCENE_DISCOVER ||
		scene > base.EligibilityScene_ELIGIBILITY_SCENE_BOOK {
		return fmt.Errorf("scene 非法")
	}
	if combination != base.RuleCombination_RULE_COMBINATION_ALL &&
		combination != base.RuleCombination_RULE_COMBINATION_ANY {
		return fmt.Errorf("combination 非法")
	}
	if err := ValidatePolicyRules(rules); err != nil {
		return err
	}
	if len(rules) == 0 {
		// server_v2 ValidatePolicy：scene 与 rules 必填（种子空规则策略走 SQL 直插，不经 console）
		return fmt.Errorf("rules 不能为空")
	}
	if _, err := ValidateRequiredFacts(requiredFacts); err != nil {
		return err
	}
	return nil
}

// SeverityScoreOf 风控 severity → 风险分（D4：LOW 30/MEDIUM 60/HIGH 90；未知 0）
func SeverityScoreOf(severity string) int64 {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "LOW":
		return 30
	case "MEDIUM":
		return 60
	case "HIGH":
		return 90
	default:
		return 0
	}
}
