package task

import (
	"testing"
	"time"

	taskpb "gacha_srv/app/task/task"
)

// 语义逐条移植自 server_v2 app/service/task/internal/biz/member_test.go

func TestFreePassAndMemberFollowDrawSpendAndSignIn(t *testing.T) {
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	board, err := NewBoard([]Definition{{
		ID: 8, Version: 1, Name: "sign-in", Period: taskpb.TaskPeriod_TASK_PERIOD_DAILY, Metric: taskpb.TaskMetric_TASK_METRIC_SIGN_IN,
		Target: 1, CycleLimit: 1, RewardTemplateID: "coupon-sign", RewardCount: 1,
	}, {
		ID: 9, Version: 1, Name: "spend", Period: taskpb.TaskPeriod_TASK_PERIOD_ONCE, Metric: taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR,
		Target: 100, CycleLimit: 1, RewardTemplateID: "coupon-spend", RewardCount: 1,
	}}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err := board.SetMemberPlan([]MemberTier{{Level: 1, SpendMinor: 100, RewardTemplateID: "member-1", RewardCount: 1}, {Level: 2, SpendMinor: 300, RewardTemplateID: "member-2", RewardCount: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := board.AddPass(PassSeason{
		ID: 4, Version: 1, Name: "hex", XPPerLevel: 100, MaxLevel: 3, XPPerSpendMinor: 1,
		PremiumLine: taskpb.PassPremiumLine_PASS_PREMIUM_LINE_MEMBER, PremiumMemberLevel: 1,
		Rewards: []PassReward{
			{Level: 1, Track: taskpb.PassTrack_PASS_TRACK_FREE, RewardTemplateID: "free-1", RewardCount: 1},
			{Level: 1, Track: taskpb.PassTrack_PASS_TRACK_PREMIUM, RewardTemplateID: "paid-1", RewardCount: 1},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := board.Apply(5, "draw-1", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 100); err != nil {
		t.Fatal(err)
	}
	if err := board.Apply(5, "draw-1", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 100); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := board.SignIn(5, 8, 1); err != nil {
		t.Fatal(err)
	}
	level, spend := board.MemberLevel(5)
	if level != 1 || spend != 100 {
		t.Fatalf("member level=%d spend=%d", level, spend)
	}
	free, err := board.ClaimPass(5, 4, 1, taskpb.PassTrack_PASS_TRACK_FREE, "free")
	if err != nil || free.RewardTemplateID != "free-1" {
		t.Fatalf("free %+v err=%v", free, err)
	}
	paid, err := board.ClaimPass(5, 4, 1, taskpb.PassTrack_PASS_TRACK_PREMIUM, "paid")
	if err != nil || paid.RewardTemplateID != "paid-1" {
		t.Fatalf("paid line %+v err=%v", paid, err)
	}
	benefit, err := board.ClaimMember(5, 1, "member-claim")
	if err != nil || benefit.RewardTemplateID != "member-1" {
		t.Fatalf("member %+v err=%v", benefit, err)
	}
}
