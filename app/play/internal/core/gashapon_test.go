package core

import (
	"testing"
)

// 案例移植自 server_v2 biz/gashapon_test.go（同款 fixture 语义）：
// LAST 全给清空者 / MVP 并列按 userId / 分阶段阈值 / 指派取首单元 / 指派须触及阶段区间。

func TestGashaponLastPrizeGoesToEmptyingUser(t *testing.T) {
	prizes := []GashaponSpecialPrize{{PrizeIndex: 0, SpecialKind: GashaponKindLast, PrizeKind: 1, CashFen: 91, Quantity: 2}}
	// 第一抽（用户 11，1 票，剩 1）：未清空 → 无奖
	state := &GashaponBoxState{
		Participants: map[int64]int64{11: 1},
		Records:      []GashaponParticipation{{UserID: 11, RequestID: "g-1", Count: 1}},
		Sent:         []int32{0},
	}
	staged := StagedGashaponAwards(state, prizes, 2, 1, []byte("seed"), nil)
	if len(staged) != 0 {
		t.Fatalf("last prize leaked before empty: %+v", staged)
	}
	// 第二抽（用户 22 清空）：终局 LAST 两单元全给清空者
	state.Participants[22] = 1
	state.Records = append(state.Records, GashaponParticipation{UserID: 22, RequestID: "g-2", Count: 1})
	final := FinalGashaponAwards(state, prizes, 22, []byte("seed"), nil)
	if len(final) != 2 || final[0].UserID != 22 || final[1].UserID != 22 || final[0].CashFen != 91 {
		t.Fatalf("last prize was not transferred in full: %+v", final)
	}
	if state.Sent[0] != 2 {
		t.Fatalf("sent not advanced: %+v", state.Sent)
	}
}

func TestGashaponMVPTruncatesTiedUsersByStableID(t *testing.T) {
	prizes := []GashaponSpecialPrize{
		{PrizeIndex: 0, SpecialKind: GashaponKindMVP, PrizeKind: 1, CashFen: 81, Quantity: 1},
		{PrizeIndex: 1, SpecialKind: GashaponKindMVP, PrizeKind: 2, PointsAmount: 82, Quantity: 3},
	}
	state := &GashaponBoxState{
		Participants: map[int64]int64{30: 1, 20: 1}, // 并列 MVP
		Records: []GashaponParticipation{
			{UserID: 30, RequestID: "m-1", Count: 1},
			{UserID: 20, RequestID: "m-2", Count: 1},
		},
		Sent: []int32{0, 0},
	}
	final := FinalGashaponAwards(state, prizes, 20, []byte("seed"), nil)
	if len(final) != 3 { // 并列 2 人：奖 1（q1）→20；奖 2（q3）→[20,30] 后第 3 单元无人（超出并列人数）截断
		t.Fatalf("MVP truncation lost: %+v", final)
	}
	if final[0].CashFen != 81 || final[0].UserID != 20 {
		t.Fatalf("single MVP slot should go to the lower user id: %+v", final[0])
	}
	if final[1].UserID != 20 || final[2].UserID != 30 {
		t.Fatalf("tied MVP users were not assigned by user id: %+v", final[1:])
	}
}

func TestGashaponStagedLuckyUsesCurrentTicketsAndBoxTotal(t *testing.T) {
	prizes := []GashaponSpecialPrize{
		{PrizeIndex: 0, SpecialKind: GashaponKindLucky, PrizeKind: 1, CashFen: 71, Quantity: 1, StageOrder: 1},
		{PrizeIndex: 1, SpecialKind: GashaponKindLucky, PrizeKind: 1, CashFen: 72, Quantity: 1, StageOrder: 2},
		{PrizeIndex: 2, SpecialKind: GashaponKindLucky, PrizeKind: 1, CashFen: 73, Quantity: 1}, // 不分阶段
	}
	// 第一抽（用户 11，2 票，剩 2）：drawn=2，T1=(1)*4/2=2 → 首阶段触发，票据 1..2 全属 11
	state := &GashaponBoxState{
		Participants: map[int64]int64{11: 2},
		Records:      []GashaponParticipation{{UserID: 11, RequestID: "s-1", Count: 2}},
		Sent:         []int32{0, 0, 0},
	}
	staged := StagedGashaponAwards(state, prizes, 4, 2, []byte("seed"), nil)
	if len(staged) != 1 || staged[0].CashFen != 71 || staged[0].UserID != 11 || staged[0].StageOrder != 1 {
		t.Fatalf("first stage should resolve inside the current ticket range: %+v", staged)
	}
	// 第二抽（用户 22，2 票，清空）：drawn=4，T2=(2)*4/2=4 → 二阶段触发，票据 3..4 全属 22；
	// 终局：LUCKY 剩余 1 单元按参与数加权（11/22 各 2 票 → 二选一）
	state.Participants[22] = 2
	state.Records = append(state.Records, GashaponParticipation{UserID: 22, RequestID: "s-2", Count: 2})
	staged2 := StagedGashaponAwards(state, prizes, 4, 4, []byte("seed"), nil)
	final := FinalGashaponAwards(state, prizes, 22, []byte("seed"), nil)
	all := append(append([]GashaponAward{}, staged2...), final...)
	if len(all) != 2 { // 二阶段 1 + 终局 LUCKY 剩余 1
		t.Fatalf("expected two staged awards and one unstaged lucky: %+v", all)
	}
	if all[0].CashFen != 72 || all[0].UserID != 22 {
		t.Fatalf("second stage should resolve in the emptying tickets: %+v", all[0])
	}
	if all[1].CashFen != 73 || (all[1].UserID != 11 && all[1].UserID != 22) {
		t.Fatalf("unstaged lucky leaked or used a stranger: %+v", all[1])
	}
}

func TestAssignedLuckyTakesTheFirstRemainingUnit(t *testing.T) {
	prizes := []GashaponSpecialPrize{{PrizeIndex: 0, SpecialKind: GashaponKindLucky, PrizeKind: 1, CashFen: 91, Quantity: 2}}
	fresh := func() *GashaponBoxState {
		return &GashaponBoxState{Participants: map[int64]int64{11: 1, 22: 1}, Sent: []int32{0}}
	}
	plain := FinalGashaponAwards(fresh(), prizes, 22, []byte("seed"), nil)
	assigned := FinalGashaponAwards(fresh(), prizes, 22, []byte("seed"), map[int32]int64{0: 11})
	if len(assigned) != 2 || assigned[0].UserID != 11 || assigned[1].UserID != plain[1].UserID {
		t.Fatalf("first unit was not reserved: plain=%+v assigned=%+v", plain, assigned)
	}
	missed := FinalGashaponAwards(fresh(), prizes, 22, []byte("seed"), map[int32]int64{0: 99})
	if missed[0].UserID != plain[0].UserID || missed[1].UserID != plain[1].UserID {
		t.Fatalf("absent assignee changed the draw: plain=%+v missed=%+v", plain, missed)
	}
}

func TestStagedLuckyAssigneeMustFallInsideTheStage(t *testing.T) {
	records := []GashaponParticipation{{UserID: 11, Count: 3}, {UserID: 22, Count: 1}}
	fresh := func(sentLen int32) *GashaponBoxState {
		copied := append([]GashaponParticipation(nil), records...)
		return &GashaponBoxState{Participants: map[int64]int64{11: 3, 22: 1}, Records: copied, Sent: make([]int32, sentLen)}
	}
	// drawn=2、q2：首单元 T=(1)*4/2=2，区间 [1..2] 全属 11 → 指派 22 触及不到区间，落回票据主人 11
	early := []GashaponSpecialPrize{{PrizeIndex: 0, SpecialKind: GashaponKindLucky, PrizeKind: 1, CashFen: 91, Quantity: 2, StageOrder: 1}}
	outside := StagedGashaponAwards(fresh(1), early, 4, 2, []byte("seed"), map[int32]int64{0: 22})
	if len(outside) != 1 || outside[0].UserID != 11 {
		t.Fatalf("assignee outside the first stage still won: %+v", outside)
	}
	// drawn=4、q1：T=(1)*4/1=4，区间 [1..4] 包含 22 的票据 4 → 指派者必中（多种 seed 验证）
	whole := []GashaponSpecialPrize{{PrizeIndex: 0, SpecialKind: GashaponKindLucky, PrizeKind: 1, CashFen: 91, Quantity: 1, StageOrder: 1}}
	var separated bool
	for _, seed := range []string{"seed", "seed-2", "seed-3", "alpha", "beta"} {
		plain := StagedGashaponAwards(fresh(1), whole, 4, 4, []byte(seed), nil)
		assigned := StagedGashaponAwards(fresh(1), whole, 4, 4, []byte(seed), map[int32]int64{0: 22})
		if len(assigned) != 1 || assigned[0].UserID != 22 {
			t.Fatalf("in-range assignee lost: %+v", assigned)
		}
		if len(plain) == 1 && plain[0].UserID != 22 {
			separated = true
		}
	}
	if !separated {
		t.Fatal("fixture never separated the assignee from the ticket owner")
	}
}
