package task

import (
	"testing"
	"time"

	taskpb "gacha_srv/app/task/task"
)

// 以下 4 例语义逐条移植自 server_v2 app/service/task/internal/biz/task_test.go

func TestProgressAddsAmountAndCycleLimitAlwaysApplies(t *testing.T) {
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	board, err := NewBoard([]Definition{{
		ID: 1, Version: 1, Name: "spend", Period: taskpb.TaskPeriod_TASK_PERIOD_DAILY, Metric: taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR,
		Target: 100, CycleLimit: 2, RewardTemplateID: "coupon-spend", RewardCount: 1,
	}}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err := board.Apply(7, "event-1", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 250); err != nil {
		t.Fatal(err)
	}
	if err := board.Apply(7, "event-1", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 250); err != nil {
		t.Fatal(err)
	}
	views, _, err := board.List(7, taskpb.TaskPeriod_TASK_PERIOD_DAILY, 20, "")
	if err != nil || len(views) != 1 || views[0].Claimed != 0 || views[0].State != taskpb.TaskProgressState_TASK_PROGRESS_STATE_CLAIMABLE {
		t.Fatalf("before claim %+v err=%v", views, err)
	}
	first, err := board.Claim(7, 1, 1, "claim-1")
	if err != nil || first.Claimed != 1 || first.RewardTemplateID != "coupon-spend" {
		t.Fatalf("first %+v err=%v", first, err)
	}
	again, err := board.Claim(7, 1, 1, "claim-1")
	if err != nil || again.Claimed != 1 {
		t.Fatalf("replay %+v err=%v", again, err)
	}
	second, err := board.Claim(7, 1, 1, "claim-2")
	if err != nil || second.Claimed != 2 {
		t.Fatalf("second %+v err=%v", second, err)
	}
	if _, err := board.Claim(7, 1, 1, "claim-3"); err != ErrTaskFinished {
		t.Fatalf("limit err=%v", err)
	}
	views, _, err = board.List(7, taskpb.TaskPeriod_TASK_PERIOD_DAILY, 20, "")
	if err != nil || len(views) != 1 || views[0].Progress != 250 || views[0].State != taskpb.TaskProgressState_TASK_PROGRESS_STATE_FINISHED {
		t.Fatalf("views %+v err=%v", views, err)
	}
}

func TestPaidDrawSpendIsManualClaim(t *testing.T) {
	board, err := NewBoard([]Definition{{
		ID: 1, Version: 1, Name: "spend", Period: taskpb.TaskPeriod_TASK_PERIOD_ONCE, Metric: taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR,
		Target: 100, CycleLimit: 1, RewardTemplateID: "coupon-spend", RewardCount: 1,
	}}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := board.ApplyPaidSpend(7, 90, "gacha_draw", 100, "CNY"); err != nil {
		t.Fatal(err)
	}
	if err := board.ApplyPaidSpend(7, 90, "gacha_draw", 100, "CNY"); err != nil {
		t.Fatal(err)
	}
	if err := board.ApplyPaidSpend(7, 91, "memory_match", 50, "CNY"); err != nil {
		t.Fatal(err)
	}
	if err := board.ApplyPaidSpend(7, 92, "wallet_topup", 500, "CNY"); err != nil {
		t.Fatal(err)
	}
	views, _, err := board.List(7, taskpb.TaskPeriod_TASK_PERIOD_ONCE, 20, "")
	if err != nil || len(views) != 1 || views[0].Progress != 150 || views[0].Claimed != 0 || views[0].State != taskpb.TaskProgressState_TASK_PROGRESS_STATE_CLAIMABLE {
		t.Fatalf("views %+v err=%v", views, err)
	}
}

func TestListDoesNotWriteAndPages(t *testing.T) {
	board, err := NewBoard([]Definition{
		{ID: 2, Version: 1, Name: "login", Period: taskpb.TaskPeriod_TASK_PERIOD_ONCE, Metric: taskpb.TaskMetric_TASK_METRIC_LOGIN, Target: 1, CycleLimit: 1, RewardTemplateID: "coupon-login", RewardCount: 1},
		{ID: 3, Version: 1, Name: "draws", Period: taskpb.TaskPeriod_TASK_PERIOD_WEEKLY, Metric: taskpb.TaskMetric_TASK_METRIC_DRAW_COUNT, Target: 3, CycleLimit: 1, RewardTemplateID: "coupon-draw", RewardCount: 1},
	}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	page, next, err := board.List(9, taskpb.TaskPeriod_TASK_PERIOD_UNSPECIFIED, 1, "")
	if err != nil || len(page) != 1 || page[0].Definition.ID != 2 || page[0].Progress != 0 || next != "2" {
		t.Fatalf("page %+v next=%s err=%v", page, next, err)
	}
	page, next, err = board.List(9, taskpb.TaskPeriod_TASK_PERIOD_UNSPECIFIED, 1, next)
	if err != nil || len(page) != 1 || page[0].Definition.ID != 3 || next != "" {
		t.Fatalf("next page %+v next=%s err=%v", page, next, err)
	}
}

func TestSignInStreakIsSeparateFromLogin(t *testing.T) {
	day := time.Date(2026, 9, 23, 2, 0, 0, 0, time.UTC)
	now := day
	board, err := NewBoard([]Definition{{
		ID: 8, Version: 1, Name: "sign-in", Period: taskpb.TaskPeriod_TASK_PERIOD_DAILY, Metric: taskpb.TaskMetric_TASK_METRIC_SIGN_IN,
		Target: 1, CycleLimit: 1, RewardTemplateID: "coupon-sign", RewardCount: 1,
	}}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err := board.Apply(4, "login-1", taskpb.TaskMetric_TASK_METRIC_SIGN_IN, 1); err != ErrInvalidTask {
		t.Fatalf("external sign-in accepted: %v", err)
	}
	streak, date, again, err := board.SignIn(4, 8, 1)
	if err != nil || streak != 1 || date != "2026-09-23" || again {
		t.Fatalf("first streak=%d date=%s again=%v err=%v", streak, date, again, err)
	}
	streak, _, again, err = board.SignIn(4, 8, 1)
	if err != nil || streak != 1 || !again {
		t.Fatalf("same day streak=%d again=%v err=%v", streak, again, err)
	}
	views, _, err := board.List(4, taskpb.TaskPeriod_TASK_PERIOD_DAILY, 20, "")
	if err != nil || len(views) != 1 || views[0].Claimed != 0 || views[0].State != taskpb.TaskProgressState_TASK_PROGRESS_STATE_CLAIMABLE {
		t.Fatalf("sign-in before claim %+v err=%v", views, err)
	}
	claim, err := board.Claim(4, 8, 1, "sign-claim")
	if err != nil || claim.Claimed != 1 {
		t.Fatalf("claim %+v err=%v", claim, err)
	}
	now = day.Add(24 * time.Hour)
	streak, date, again, err = board.SignIn(4, 8, 1)
	if err != nil || streak != 2 || date != "2026-09-24" || again {
		t.Fatalf("next day streak=%d date=%s again=%v err=%v", streak, date, again, err)
	}
	now = day.Add(72 * time.Hour)
	streak, _, _, err = board.SignIn(4, 8, 1)
	if err != nil || streak != 1 {
		t.Fatalf("gap streak=%d err=%v", streak, err)
	}
}
