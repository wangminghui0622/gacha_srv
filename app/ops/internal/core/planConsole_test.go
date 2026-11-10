package core

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"gacha_srv/app/ops/ops"
)

// bootstrapSeedHash 与 20_gacha_ops_m14e.sql 种子 UNHEX 一致（哈希契约锁死）。
const bootstrapSeedHash = "c4ea9a722bcccc403081206cf2e0ad6c9525f8452f2891da3d6bbecaa4d7fe06"

func legacyTask(id uint64) *ops.TaskDefinition {
	return &ops.TaskDefinition{
		TaskId: id, Version: 1, Name: "任务", Period: ops.TaskPeriod_TASK_PERIOD_DAILY,
		Metric: ops.TaskMetric_TASK_METRIC_DRAW_COUNT, Target: 1, CycleLimit: 0,
		RewardTemplateId: "T-1", RewardCount: 1,
	}
}

func structuredTask(id uint64) *ops.TaskDefinition {
	t := legacyTask(id)
	t.RewardTemplateId = ""
	t.RewardCount = 0
	t.Reward = &ops.TaskRewardDefinition{
		Count:  1,
		Target: &ops.TaskRewardDefinition_Coupon{Coupon: &ops.CouponRewardReference{TemplateId: 7, TemplateVersion: 3}},
	}
	return t
}

func TestBootstrapPlanHashMatchesSQLSeed(t *testing.T) {
	hash, err := TaskPlanConfigHash(BootstrapTaskPlan())
	if err != nil {
		t.Fatalf("bootstrap plan 校验失败: %v", err)
	}
	want, _ := hex.DecodeString(bootstrapSeedHash)
	if !bytes.Equal(hash, want) {
		t.Fatalf("bootstrap 哈希漂移: got %x want %s（SQL 种子与运行时不同源！）", hash, bootstrapSeedHash)
	}
}

func TestValidateTaskPlan(t *testing.T) {
	cases := []struct {
		name    string
		plan    *ops.TaskPlan
		wantErr string // 空 = 期望通过
	}{
		{"空计划", &ops.TaskPlan{}, "至少包含"},
		{"nil", nil, "至少包含"},
		{"合法 legacy", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{legacyTask(1), legacyTask(2)}}, ""},
		{"合法 structured coupon", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{structuredTask(1)}}, ""},
		{"structured inventory", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{{
			TaskId: 1, Version: 1, Name: "任务", Period: ops.TaskPeriod_TASK_PERIOD_ONCE,
			Metric: ops.TaskMetric_TASK_METRIC_RECHARGE_MINOR, Target: 100,
			Reward: &ops.TaskRewardDefinition{
				Count: 5,
				Target: &ops.TaskRewardDefinition_Inventory{
					Inventory: &ops.InventoryRewardReference{ProductId: 1, SkuId: 2, ProductVersion: 3},
				},
			},
		}}}, ""},
		{"任务 id=0", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{legacyTask(0)}}, "非法"},
		{"任务 version=0", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition { x := legacyTask(1); x.Version = 0; return x }()}}, "非法"},
		{"target=0", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition { x := legacyTask(1); x.Target = 0; return x }()}}, "非法"},
		{"metric 未指定", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition { x := legacyTask(1); x.Metric = ops.TaskMetric_TASK_METRIC_UNSPECIFIED; return x }()}}, "非法"},
		{"period 未指定", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition { x := legacyTask(1); x.Period = ops.TaskPeriod_TASK_PERIOD_UNSPECIFIED; return x }()}}, "非法"},
		{"任务 id 重复", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{legacyTask(1), legacyTask(1)}}, "重复"},
		{"legacy count=0", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition { x := legacyTask(1); x.RewardCount = 0; return x }()}}, "非法"},
		{"legacy count>1000", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition { x := legacyTask(1); x.RewardCount = 1001; return x }()}}, "非法"},
		{"structured 混 legacy 字段", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition { x := structuredTask(1); x.RewardCount = 1; return x }()}}, "非法"},
		{"structured coupon 引用不完整", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition {
			x := structuredTask(1)
			x.Reward.Target = &ops.TaskRewardDefinition_Coupon{Coupon: &ops.CouponRewardReference{TemplateId: 7}}
			return x
		}()}}, "非法"},
		{"structured oneof 空", &ops.TaskPlan{Tasks: []*ops.TaskDefinition{func() *ops.TaskDefinition {
			x := structuredTask(1)
			x.Reward.Target = nil
			return x
		}()}}, "非法"},
		{"pass 合法", &ops.TaskPlan{Passes: []*ops.PassSeason{{
			PassId: 9, Version: 1, Name: "赛季", XpPerLevel: 100, MaxLevel: 10,
			XpPerSpendMinor: 1, PremiumLine: ops.PassPremiumLine_PASS_PREMIUM_LINE_PAYMENT,
			Rewards: []*ops.PassReward{
				{Level: 1, Track: ops.PassTrack_PASS_TRACK_FREE, RewardTemplateId: "T-1", RewardCount: 1},
				{Level: 1, Track: ops.PassTrack_PASS_TRACK_PREMIUM, RewardTemplateId: "T-2", RewardCount: 1},
			},
		}}}, ""},
		{"pass maxLevel=0", &ops.TaskPlan{Passes: []*ops.PassSeason{func() *ops.PassSeason {
			p := &ops.PassSeason{PassId: 9, Version: 1, XpPerLevel: 100, MaxLevel: 0, XpPerSpendMinor: 1,
				Rewards: []*ops.PassReward{{Level: 1, Track: ops.PassTrack_PASS_TRACK_FREE, RewardTemplateId: "T-1", RewardCount: 1}}}
			return p
		}()}}, "非法"},
		{"pass 奖励空", &ops.TaskPlan{Passes: []*ops.PassSeason{{PassId: 9, Version: 1, XpPerLevel: 100, MaxLevel: 5, XpPerSpendMinor: 1}}}, "非法"},
		{"pass 奖励 level>max", &ops.TaskPlan{Passes: []*ops.PassSeason{{PassId: 9, Version: 1, XpPerLevel: 100, MaxLevel: 5, XpPerSpendMinor: 1,
			Rewards: []*ops.PassReward{{Level: 6, Track: ops.PassTrack_PASS_TRACK_FREE, RewardTemplateId: "T-1", RewardCount: 1}}}}}, "非法"},
		{"pass 奖励重复 level:track", &ops.TaskPlan{Passes: []*ops.PassSeason{{PassId: 9, Version: 1, XpPerLevel: 100, MaxLevel: 5, XpPerSpendMinor: 1,
			Rewards: []*ops.PassReward{
				{Level: 2, Track: ops.PassTrack_PASS_TRACK_FREE, RewardTemplateId: "T-1", RewardCount: 1},
				{Level: 2, Track: ops.PassTrack_PASS_TRACK_FREE, RewardTemplateId: "T-2", RewardCount: 1},
			}}}}, "重复"},
		{"pass premium 奖励但 line=UNSPECIFIED", &ops.TaskPlan{Passes: []*ops.PassSeason{{PassId: 9, Version: 1, XpPerLevel: 100, MaxLevel: 5, XpPerSpendMinor: 1,
			Rewards: []*ops.PassReward{{Level: 1, Track: ops.PassTrack_PASS_TRACK_PREMIUM, RewardTemplateId: "T-1", RewardCount: 1}}}}}, "非法"},
		{"member tiers 合法", &ops.TaskPlan{MemberTiers: []*ops.MemberTier{
			{Level: 1, SpendMinor: 100, RewardTemplateId: "T-1", RewardCount: 1},
			{Level: 2, SpendMinor: 500, RewardTemplateId: "T-2", RewardCount: 2},
		}}, ""},
		{"member tiers spend 非递增", &ops.TaskPlan{MemberTiers: []*ops.MemberTier{
			{Level: 1, SpendMinor: 100, RewardTemplateId: "T-1", RewardCount: 1},
			{Level: 2, SpendMinor: 100, RewardTemplateId: "T-2", RewardCount: 2},
		}}, "非法"},
		{"member tiers level 重复", &ops.TaskPlan{MemberTiers: []*ops.MemberTier{
			{Level: 1, SpendMinor: 100, RewardTemplateId: "T-1", RewardCount: 1},
			{Level: 1, SpendMinor: 500, RewardTemplateId: "T-2", RewardCount: 2},
		}}, "重复"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTaskPlan(tc.plan)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("期望通过，得到 %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("期望错误含 %q，得到 %v", tc.wantErr, err)
			}
		})
	}
}

func TestHasStructuredRewards(t *testing.T) {
	if HasStructuredRewards(&ops.TaskPlan{Tasks: []*ops.TaskDefinition{legacyTask(1)}}) {
		t.Fatal("legacy 计划不应有 structured reward")
	}
	if !HasStructuredRewards(&ops.TaskPlan{Tasks: []*ops.TaskDefinition{structuredTask(1)}}) {
		t.Fatal("structured 任务应被检出")
	}
	if !HasStructuredRewards(&ops.TaskPlan{Passes: []*ops.PassSeason{{
		PassId: 9, Version: 1, XpPerLevel: 100, MaxLevel: 5, XpPerSpendMinor: 1,
		Rewards: []*ops.PassReward{{
			Level: 1, Track: ops.PassTrack_PASS_TRACK_FREE,
			Reward: &ops.TaskRewardDefinition{Count: 1, Target: &ops.TaskRewardDefinition_Coupon{
				Coupon: &ops.CouponRewardReference{TemplateId: 1, TemplateVersion: 1}}},
		}},
	}}}) {
		t.Fatal("pass 的 structured reward 应被检出")
	}
	if !HasStructuredRewards(&ops.TaskPlan{MemberTiers: []*ops.MemberTier{{
		Level: 1, SpendMinor: 100,
		Reward: &ops.TaskRewardDefinition{Count: 1, Target: &ops.TaskRewardDefinition_Inventory{
			Inventory: &ops.InventoryRewardReference{ProductId: 1, SkuId: 1, ProductVersion: 1}}},
	}}}) {
		t.Fatal("member tier 的 structured reward 应被检出")
	}
}

func TestTaskPlanConfigHashDeterministic(t *testing.T) {
	plan := &ops.TaskPlan{Tasks: []*ops.TaskDefinition{legacyTask(1), legacyTask(2)}}
	a, err := TaskPlanConfigHash(plan)
	if err != nil {
		t.Fatalf("hash 失败: %v", err)
	}
	b, err := TaskPlanConfigHash(plan)
	if err != nil {
		t.Fatalf("hash 失败: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("同计划两次哈希应一致")
	}
	// 状态/时间戳字段不参与哈希（canonical 清零）——克隆后改动应不影响
	plan.Status = ops.TaskPlanStatus_TASK_PLAN_STATUS_PUBLISHED
	plan.Version = 42
	plan.CreatedAtUnixMillis = 12345
	c, err := TaskPlanConfigHash(plan)
	if err != nil {
		t.Fatalf("hash 失败: %v", err)
	}
	if !bytes.Equal(a, c) {
		t.Fatal("version/status/时间戳不应参与哈希")
	}
	// 顺序变化 → 不同哈希（canonical 对提交顺序确定）
	reordered := &ops.TaskPlan{Tasks: []*ops.TaskDefinition{legacyTask(2), legacyTask(1)}}
	d, err := TaskPlanConfigHash(reordered)
	if err != nil {
		t.Fatalf("hash 失败: %v", err)
	}
	if bytes.Equal(a, d) {
		t.Fatal("任务顺序变化应改变哈希")
	}
}
