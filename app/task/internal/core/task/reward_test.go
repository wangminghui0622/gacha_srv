package task

import (
	"bytes"
	"testing"

	"gacha_srv/app/ops/ops"
)

// 语义逐条移植自 server_v2 app/service/task/internal/biz/reward_test.go

func TestStructuredRewardPlanValidation(t *testing.T) {
	reward := &ops.TaskRewardDefinition{Count: 2, Target: &ops.TaskRewardDefinition_Coupon{Coupon: &ops.CouponRewardReference{TemplateId: 7, TemplateVersion: 1}}}
	task := &ops.TaskDefinition{TaskId: 1, Version: 1, Period: ops.TaskPeriod_TASK_PERIOD_ONCE, Metric: ops.TaskMetric_TASK_METRIC_DRAW_COUNT, Target: 1, Reward: reward}
	plan := &ops.TaskPlan{Tasks: []*ops.TaskDefinition{task}}
	first, err := PlanHash(plan)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := SnapshotFromOpsPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	reward.GetCoupon().TemplateVersion = 2
	if snapshot.Tasks[0].Reward.GetCoupon().TemplateVersion != 1 {
		t.Fatal("reward snapshot aliases input")
	}
	second, err := PlanHash(plan)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("reward version absent from hash")
	}
	task.RewardTemplateId = "ambiguous"
	if _, err = PlanHash(plan); err == nil {
		t.Fatal("mixed legacy and structured definition accepted")
	}
	task.RewardTemplateId = ""
	reward.GetCoupon().TemplateVersion = 0
	if _, err = PlanHash(plan); err == nil {
		t.Fatal("unversioned coupon accepted")
	}
	reward.Target = &ops.TaskRewardDefinition_Inventory{Inventory: &ops.InventoryRewardReference{ProductId: 1, SkuId: 2, ProductVersion: 3}}
	if _, err = PlanHash(plan); err != nil {
		t.Fatal(err)
	}
	reward.Count = 1001
	if _, err = PlanHash(plan); err == nil {
		t.Fatal("excess reward count accepted")
	}
}
