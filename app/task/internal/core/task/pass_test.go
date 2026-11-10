package task

import (
	"testing"
	"time"

	taskpb "gacha_srv/app/task/task"
)

// 以下 2 例语义逐条移植自 server_v2 app/service/task/internal/biz/pass_test.go

func TestPassLevelsAndPremiumTrack(t *testing.T) {
	board, err := NewBoard(nil, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	err = board.AddPass(PassSeason{
		ID: 1, Version: 1, Name: "hex", XPPerLevel: 100, MaxLevel: 2, XPPerSpendMinor: 1,
		PremiumLine: taskpb.PassPremiumLine_PASS_PREMIUM_LINE_MEMBER, PremiumMemberLevel: 2,
		Rewards: []PassReward{
			{Level: 1, Track: taskpb.PassTrack_PASS_TRACK_FREE, RewardTemplateID: "free-1", RewardCount: 1},
			{Level: 1, Track: taskpb.PassTrack_PASS_TRACK_PREMIUM, RewardTemplateID: "paid-1", RewardCount: 1},
			{Level: 2, Track: taskpb.PassTrack_PASS_TRACK_FREE, RewardTemplateID: "free-2", RewardCount: 1},
			{Level: 2, Track: taskpb.PassTrack_PASS_TRACK_PREMIUM, RewardTemplateID: "paid-2", RewardCount: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := board.SetMemberPlan([]MemberTier{
		{Level: 1, SpendMinor: 100, RewardTemplateID: "member-1", RewardCount: 1},
		{Level: 2, SpendMinor: 300, RewardTemplateID: "member-2", RewardCount: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := board.Apply(3, "spend-1", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 150); err != nil {
		t.Fatal(err)
	}
	if err := board.Apply(3, "spend-1", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 150); err != nil {
		t.Fatal(err)
	}
	free, err := board.ClaimPass(3, 1, 1, taskpb.PassTrack_PASS_TRACK_FREE, "claim-free")
	if err != nil || free.RewardTemplateID != "free-1" {
		t.Fatalf("free %+v err=%v", free, err)
	}
	if _, err := board.ClaimPass(3, 1, 1, taskpb.PassTrack_PASS_TRACK_PREMIUM, "claim-paid"); err != ErrTaskNotReady {
		t.Fatalf("premium before member line: %v", err)
	}
	if err := board.UnlockPass(3, 1); err != ErrInvalidTask {
		t.Fatalf("payment unlock on member line: %v", err)
	}
	if _, err := board.ClaimPass(3, 1, 2, taskpb.PassTrack_PASS_TRACK_FREE, "claim-two"); err != ErrTaskNotReady {
		t.Fatalf("level 2 early: %v", err)
	}
	if err := board.Apply(3, "spend-2", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 200); err != nil {
		t.Fatal(err)
	}
	paid, err := board.ClaimPass(3, 1, 2, taskpb.PassTrack_PASS_TRACK_PREMIUM, "claim-paid-2")
	if err != nil || paid.RewardTemplateID != "paid-2" {
		t.Fatalf("paid %+v err=%v", paid, err)
	}
	if err := board.Apply(3, "spend-3", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 500); err != nil {
		t.Fatal(err)
	}
	free2, err := board.ClaimPass(3, 1, 2, taskpb.PassTrack_PASS_TRACK_FREE, "claim-free-2")
	if err != nil || free2.RewardTemplateID != "free-2" {
		t.Fatalf("capped free %+v err=%v", free2, err)
	}
}

func TestPaidPassUnlocksWithoutRaisingMember(t *testing.T) {
	board, err := NewBoard(nil, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	err = board.AddPass(PassSeason{
		ID: 2, Version: 1, Name: "paid", XPPerLevel: 100, MaxLevel: 1, XPPerSpendMinor: 1,
		PremiumLine: taskpb.PassPremiumLine_PASS_PREMIUM_LINE_PAYMENT,
		Rewards: []PassReward{
			{Level: 1, Track: taskpb.PassTrack_PASS_TRACK_FREE, RewardTemplateID: "free-1", RewardCount: 1},
			{Level: 1, Track: taskpb.PassTrack_PASS_TRACK_PREMIUM, RewardTemplateID: "paid-1", RewardCount: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := board.Apply(3, "spend-1", taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := board.ClaimPass(3, 2, 1, taskpb.PassTrack_PASS_TRACK_PREMIUM, "claim-paid"); err != ErrTaskNotReady {
		t.Fatalf("before payment: %v", err)
	}
	if err := board.UnlockPass(3, 2); err != nil {
		t.Fatal(err)
	}
	paid, err := board.ClaimPass(3, 2, 1, taskpb.PassTrack_PASS_TRACK_PREMIUM, "claim-paid")
	if err != nil || paid.RewardTemplateID != "paid-1" {
		t.Fatalf("paid %+v err=%v", paid, err)
	}
}
