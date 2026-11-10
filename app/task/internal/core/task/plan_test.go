package task

import (
	"bytes"
	"testing"

	"gacha_srv/app/ops/ops"
)

// 以下 2 例语义逐条移植自 server_v2 app/service/task/internal/biz/plan_test.go

func TestValidateTaskPlanHashIgnoresStorageMetadata(t *testing.T) {
	plan := &ops.TaskPlan{Version: 9, Status: ops.TaskPlanStatus_TASK_PLAN_STATUS_DRAFT, CreatedAtUnixMillis: 10, Tasks: []*ops.TaskDefinition{{TaskId: 1, Version: 1, Name: "spend", Period: ops.TaskPeriod_TASK_PERIOD_ONCE, Metric: ops.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, Target: 100, RewardTemplateId: "reward", RewardCount: 1}}}
	first, err := PlanHash(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Version = 10
	plan.Status = ops.TaskPlanStatus_TASK_PLAN_STATUS_PUBLISHED
	plan.ConfigHash = []byte("ignored")
	plan.PublishedAtUnixMillis = 20
	second, err := PlanHash(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("metadata changed hash: %x != %x", first, second)
	}
}

func TestValidateTaskPlanRejectsDuplicateTask(t *testing.T) {
	task := &ops.TaskDefinition{TaskId: 1, Version: 1, Name: "spend", Period: ops.TaskPeriod_TASK_PERIOD_ONCE, Metric: ops.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, Target: 100, RewardTemplateId: "reward", RewardCount: 1}
	if _, err := PlanHash(&ops.TaskPlan{Tasks: []*ops.TaskDefinition{task, task}}); err == nil {
		t.Fatal("expected duplicate task rejection")
	}
}
