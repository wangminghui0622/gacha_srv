package core

import (
	"strings"
	"testing"
)

// server_v2 rolling_draw_test 同款语义案例：50% 剔前缀、并集、等级判定、规则校验。

func TestParseRollingRules(t *testing.T) {
	// 空串 → 零行为默认规则
	r, err := ParseRollingRules("")
	if err != nil || r.HighValueThresholdFen != 0 || r.SingleDrawExclusionEnabled {
		t.Fatalf("empty rules want zero behavior, got %+v err=%v", r, err)
	}

	// 合法 JSON
	r, err = ParseRollingRules(`{"high_value_threshold_fen":3000,"single_draw_exclusion_enabled":true,"single_draw_excluded_percent":50,"user_excluded_percent":25}`)
	if err != nil {
		t.Fatalf("valid rules parse failed: %v", err)
	}
	if r.HighValueThresholdFen != 3000 || !r.SingleDrawExclusionEnabled ||
		r.SingleDrawExcludedPercent != 50 || r.UserExcludedPercent != 25 {
		t.Fatalf("valid rules mismatch: %+v", r)
	}

	// JSON 无效
	if _, err = ParseRollingRules(`{bad`); err == nil {
		t.Fatal("invalid JSON want error")
	}

	// percent 越界（101 / -1）→ 错误（logic 层映射 2005）
	if _, err = ParseRollingRules(`{"single_draw_excluded_percent":101}`); err == nil {
		t.Fatal("single percent 101 want error")
	}
	if _, err = ParseRollingRules(`{"user_excluded_percent":-1}`); err == nil {
		t.Fatal("user percent -1 want error")
	}
	if !strings.Contains(err.Error(), "越界") {
		t.Fatalf("error message want 越界 hint, got: %v", err)
	}
}

func TestRollingTier(t *testing.T) {
	cases := []struct {
		name      string
		e         RollingEntry
		threshold int64
		want      int32
	}{
		{"limited wins over high value", RollingEntry{Id: 1, Limited: 1, DisplayVal: 99999}, 3000, TierLimited},
		{"high value over threshold", RollingEntry{Id: 2, DisplayVal: 5000}, 3000, TierHighValue},
		{"equal threshold not high", RollingEntry{Id: 3, DisplayVal: 3000}, 3000, TierNormal},
		{"normal", RollingEntry{Id: 4, DisplayVal: 100}, 3000, TierNormal},
		{"threshold zero disables high", RollingEntry{Id: 5, DisplayVal: 5000}, 0, TierNormal},
	}
	for _, c := range cases {
		if got := RollingTier(c.e, c.threshold); got != c.want {
			t.Errorf("%s: RollingTier(%+v, %d) = %d, want %d", c.name, c.e, c.threshold, got, c.want)
		}
	}
}

func TestExcludeRollingSetPrefix(t *testing.T) {
	// id 升序（调用方契约）：4 个限量 + 2 个普通
	entries := []RollingEntry{
		{Id: 1, Limited: 1}, {Id: 2, Limited: 1}, {Id: 3, Limited: 1}, {Id: 4, Limited: 1},
		{Id: 10}, {Id: 11},
	}

	// 50% 剔前缀：ceil(4*50/100)=2 → 限量中 id 最小的 2 个被剔除（server_v2 50% 案例）
	ex := ExcludeRollingSet(entries, 50, 0)
	if len(ex) != 2 {
		t.Fatalf("50%% of 4 limited want 2 excluded, got %d: %v", len(ex), ex)
	}
	for _, id := range []int64{1, 2} {
		if _, ok := ex[id]; !ok {
			t.Errorf("50%% prefix want id %d excluded", id)
		}
	}
	if _, ok := ex[3]; ok {
		t.Error("50% prefix must not exclude id 3")
	}

	// 33% 向上取整：ceil(4*33/100)=ceil(1.32)=2
	ex = ExcludeRollingSet(entries, 33, 0)
	if len(ex) != 2 {
		t.Fatalf("33%% of 4 want 2 (ceiling), got %d", len(ex))
	}

	// 100% → 全剔除；0% → 不剔除
	if ex = ExcludeRollingSet(entries, 100, 0); len(ex) != 4 {
		t.Fatalf("100%% want 4 excluded, got %d", len(ex))
	}
	if ex = ExcludeRollingSet(entries, 0, 0); len(ex) != 0 {
		t.Fatalf("0%% want none excluded, got %d", len(ex))
	}
}

func TestExcludeRollingSetUnion(t *testing.T) {
	// 并集：单抽 50%（前缀 2）+ 用户 25%（前缀 1）→ 并集 = 前缀 2（用户 100% 时全剔除）
	entries := []RollingEntry{
		{Id: 1, Limited: 1}, {Id: 2, Limited: 1}, {Id: 3, Limited: 1}, {Id: 4, Limited: 1},
		{Id: 10},
	}
	ex := ExcludeRollingSet(entries, 50, 25)
	if len(ex) != 2 {
		t.Fatalf("union 50%%+25%% want 2 excluded, got %d", len(ex))
	}
	ex = ExcludeRollingSet(entries, 50, 100)
	if len(ex) != 4 {
		t.Fatalf("union 50%%+100%% want 4 excluded, got %d", len(ex))
	}

	// 无限量条目 → 空集
	ex = ExcludeRollingSet([]RollingEntry{{Id: 10}}, 100, 100)
	if len(ex) != 0 {
		t.Fatalf("no limited entries want empty set, got %d", len(ex))
	}
}

func TestExcludeRollingSetAllLimitedExcluded(t *testing.T) {
	// 全限量 + 100% → 全部剔除（调用方据此判 noneLeft → 2002 售罄）
	entries := []RollingEntry{{Id: 1, Limited: 1}, {Id: 2, Limited: 1}}
	ex := ExcludeRollingSet(entries, 0, 100)
	if len(ex) != len(entries) {
		t.Fatalf("want all excluded, got %d/%d", len(ex), len(entries))
	}
}
