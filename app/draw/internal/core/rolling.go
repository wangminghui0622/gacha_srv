package core

import (
	"encoding/json"
	"fmt"
)

// M12 滚动中奖（rolling_draw）：规则解析、限量剔除、等级判定。
// 语义移植自 server_v2 rolling_draw。本仓 SettleDraw 恒单抽（无 count>1 普通盒抽，
// 见计划 D7），单抽剔除开关按规则原样生效；用户剔除比例直接读配置（本仓无风控快照链路，
// server_v2 由 gacha_rolling_draw_risk_snapshot 注入，此处为其比例化等价物）。

// 滚动中奖等级（server_v2 常量同款）
const (
	TierNormal    int32 = 1 // 普通：既非限量也未过高档阈值
	TierHighValue int32 = 2 // 高档：展示价 > 阈值
	TierLimited   int32 = 3 // 限量：rolling_limited 标记条目
)

// RollingRules 滚动规则（draw_boxes.rolling_rules_json；NULL/空 = 非滚动盒，零行为）
type RollingRules struct {
	HighValueThresholdFen      int64 `json:"high_value_threshold_fen"`      // 高档阈值（分）：display>阈值 → HIGH_VALUE；0=关闭
	SingleDrawExclusionEnabled bool  `json:"single_draw_exclusion_enabled"` // 单抽剔除开关（本仓 SettleDraw 恒单抽）
	SingleDrawExcludedPercent  int   `json:"single_draw_excluded_percent"`  // 单抽剔除限量前缀比例 0-100
	UserExcludedPercent        int   `json:"user_excluded_percent"`         // 用户剔除限量前缀比例 0-100
}

// ParseRollingRules 解析并校验规则；空串 → 零行为默认规则。
// percent 越界 / JSON 无效返回错误（logic 层映射 errorx 2005 Aborted）。
func ParseRollingRules(jsonStr string) (*RollingRules, error) {
	if jsonStr == "" {
		return &RollingRules{}, nil
	}
	r := &RollingRules{}
	if err := json.Unmarshal([]byte(jsonStr), r); err != nil {
		return nil, fmt.Errorf("滚动规则 JSON 无效: %w", err)
	}
	if r.SingleDrawExcludedPercent < 0 || r.SingleDrawExcludedPercent > 100 {
		return nil, fmt.Errorf("single_draw_excluded_percent 越界: %d", r.SingleDrawExcludedPercent)
	}
	if r.UserExcludedPercent < 0 || r.UserExcludedPercent > 100 {
		return nil, fmt.Errorf("user_excluded_percent 越界: %d", r.UserExcludedPercent)
	}
	return r, nil
}

// RollingEntry 滚动判定条目；调用方保证按 entry id 升序传入（前缀剔除依赖此序）
type RollingEntry struct {
	Id         int64
	Limited    int64 // 1=限量稀有（可被剔除、feed 显示 LIMITED）
	DisplayVal int64 // 展示价（分）
}

// RollingTier 等级判定：限量 > 高档（过阈值）> 普通；threshold<=0 关闭高档判定
func RollingTier(e RollingEntry, threshold int64) int32 {
	if e.Limited == 1 {
		return TierLimited
	}
	if threshold > 0 && e.DisplayVal > threshold {
		return TierHighValue
	}
	return TierNormal
}

// ceilPercentPrefix 前缀条数向上取整：ceil(n*p/100)；percent 越界已由 Parse 校验
func ceilPercentPrefix(n, percent int) int {
	if percent <= 0 || n <= 0 {
		return 0
	}
	if percent >= 100 {
		return n
	}
	return (n*percent + 99) / 100
}

// ExcludeRollingSet 滚动剔除集合：对 limited 条目按 id 升序取前缀 ceil(n*p/100)，
// 单抽比例与用户比例的并集（两者都取前缀，并集 = 较大前缀）。
// singlePercent 由调用方按 SingleDrawExclusionEnabled 决定是否传值。
func ExcludeRollingSet(entries []RollingEntry, singlePercent, userPercent int) map[int64]struct{} {
	limited := make([]RollingEntry, 0, len(entries))
	for _, e := range entries {
		if e.Limited == 1 {
			limited = append(limited, e)
		}
	}
	n := len(limited)
	prefix := ceilPercentPrefix(n, singlePercent)
	if k := ceilPercentPrefix(n, userPercent); k > prefix {
		prefix = k
	}
	excluded := make(map[int64]struct{}, prefix)
	for i := 0; i < prefix; i++ {
		excluded[limited[i].Id] = struct{}{}
	}
	return excluded
}
