// M14d 控制台（base 域）：政策 record 构造助手（game/campaignConsoleOps 先例）。
package logic

import (
	"encoding/json"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/repo"
)

// versionProtoOf 版本行 + 规则 → PolicyVersion 消息（required_facts 反序列化）
func versionProtoOf(v *repo.EligibilityVersionRow, rules []repo.EligibilityRuleRow) *base.PolicyVersion {
	var facts []string
	if v.RequiredFacts != "" {
		_ = json.Unmarshal([]byte(v.RequiredFacts), &facts) // 写入路径保证合法
	}
	out := &base.PolicyVersion{
		PolicyId:      uint64(v.PolicyID),
		Version:       uint64(v.Version),
		Status:        base.PolicyStatus(v.Status),
		Scene:         base.EligibilityScene(v.Scene),
		Combination:   base.RuleCombination(v.Combination),
		RequiredFacts: facts,
		ConfigHash:    append([]byte(nil), v.ConfigHash...),
	}
	for i := range rules {
		r := &rules[i]
		out.Rules = append(out.Rules, &base.EligibilityRule{
			RuleId:      r.RuleID,
			Expression:  r.Expression,
			FailureCode: r.FailureCode,
		})
	}
	return out
}

// policyRecordOf 头 + 版本行 + 规则 → EligibilityPolicyRecord
func policyRecordOf(head *repo.EligibilityHead, v *repo.EligibilityVersionRow,
	rules []repo.EligibilityRuleRow) *base.EligibilityPolicyRecord {
	return &base.EligibilityPolicyRecord{
		Policy: &base.EligibilityPolicy{
			PolicyId:       uint64(head.ID),
			TenantId:       1,
			Code:           head.PolicyCode,
			Name:           head.Name,
			CurrentVersion: uint64(head.CurrentVersion),
			LatestVersion:  uint64(head.LatestVersion),
		},
		PolicyVersion: versionProtoOf(v, rules),
	}
}

// rulesOf 请求规则 → 表行（sort_order 按请求顺序）
func rulesOf(policyID, version int64, rules []*base.EligibilityRule) []repo.EligibilityRuleRow {
	rows := make([]repo.EligibilityRuleRow, 0, len(rules))
	for i, r := range rules {
		rows = append(rows, repo.EligibilityRuleRow{
			PolicyID:      policyID,
			PolicyVersion: version,
			RuleID:        r.RuleId,
			Expression:    r.Expression,
			FailureCode:   r.FailureCode,
			SortOrder:     int32(i),
		})
	}
	return rows
}
