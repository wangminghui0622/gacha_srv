package core

import (
	"encoding/json"
	"testing"
)

func gbSlot(entryID int64, name string, remaining int32) GroupBuySlot {
	return GroupBuySlot{EntryID: entryID, EntryName: name, IsPrize: name != "谢谢参与", Remaining: remaining}
}

// gbAwardsOfKind 结算奖按 kind 过滤
func gbAwardsOfKind(awards []GroupBuyAward, kind string) []GroupBuyAward {
	out := make([]GroupBuyAward, 0)
	for _, a := range awards {
		if a.Kind == kind {
			out = append(out, a)
		}
	}
	return out
}

// TestSettleGroupBuyBatchFull 成团结算：5 槽（e1×2/e2×2/e3×1）+ e3 子奖 1 件 + 幸运奖 1 件 + 幸运子奖 1 件
// = 8 件结算奖；条目实例全消耗（槽=条目展开）；每注 ≥1 槽。
func TestSettleGroupBuyBatchFull(t *testing.T) {
	slots := []GroupBuySlot{gbSlot(1, "谢谢参与", 2), gbSlot(1, "谢谢参与", 2), gbSlot(2, "金币×50", 2), gbSlot(2, "金币×50", 2), gbSlot(3, "限定皮肤", 1)}
	children := []GroupBuyChildStock{
		{EntryID: 4, ParentID: 3, Remaining: 1, PrizeShape: PrizeShape{PrizeKind: 2, PointsAmount: 100}},
	}
	lucky := []GroupBuyLuckyStock{{
		Index: 0, Remaining: 1,
		PrizeShape: PrizeShape{PrizeKind: 1, CashFen: 1000},
		Children:   []GroupBuyLuckyChildStock{{Index: 0, PrizeShape: PrizeShape{PrizeKind: 2, PointsAmount: 50}}},
	}}
	bets := []GroupBuyBet{{UserID: 11, DrawID: "d1", Count: 1}, {UserID: 22, DrawID: "d2", Count: 2}}
	awards, err := SettleGroupBuyBatch(bets, 3, 0, slots, children, lucky, []byte("gb-seed"))
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}
	normal := gbAwardsOfKind(awards, GroupBuyAwardKindNormal)
	child := gbAwardsOfKind(awards, GroupBuyAwardKindChild)
	luckyAwards := gbAwardsOfKind(awards, GroupBuyAwardKindLucky)
	if len(normal) != 5 || len(child) != 2 || len(luckyAwards) != 1 {
		t.Fatalf("award counts mismatch: normal=%d child=%d lucky=%d (%+v)", len(normal), len(child), len(luckyAwards), awards)
	}
	// 条目实例全消耗：e1×2、e2×2、e3×1
	counts := map[int64]int{}
	for _, a := range normal {
		counts[a.EntryID]++
	}
	if counts[1] != 2 || counts[2] != 2 || counts[3] != 1 {
		t.Fatalf("normal slot distribution mismatch: %+v", counts)
	}
	// e3 子奖：父条目必然抽中（全槽消耗）→ 1 件给 e3 槽主人
	var entryChild *GroupBuyAward
	for i := range child {
		if child[i].EntryID == 4 {
			entryChild = &child[i]
		}
	}
	if entryChild == nil || entryChild.PointsAmount != 100 {
		t.Fatalf("entry child award mismatch: %+v", child)
	}
	var skinOwner int64
	for _, a := range normal {
		if a.EntryID == 3 {
			skinOwner = a.UserID
		}
	}
	if entryChild.UserID != skinOwner {
		t.Fatalf("entry child should go to parent owner: child=%+v owner=%d", entryChild, skinOwner)
	}
	// 幸运奖 + 幸运子奖同主人（条目子奖带 EntryID、幸运子奖带 LuckyIndex 区分）
	luckyAward := luckyAwards[0]
	if luckyAward.CashFen != 1000 {
		t.Fatalf("lucky award mismatch: %+v", luckyAward)
	}
	var luckyChild *GroupBuyAward
	for i := range child {
		if child[i].LuckyIndex == 0 {
			luckyChild = &child[i]
		}
	}
	if luckyChild == nil || luckyChild.PointsAmount != 50 || luckyChild.UserID != luckyAward.UserID {
		t.Fatalf("lucky child mismatch: %+v lucky=%+v", luckyChild, luckyAward)
	}
	// 幸运子奖无库存推进：重复结算仍随幸运奖发放（配置数量仅展示）
	children2 := []GroupBuyChildStock{{EntryID: 4, ParentID: 3, Remaining: 1, PrizeShape: PrizeShape{PrizeKind: 2, PointsAmount: 100}}}
	lucky2 := []GroupBuyLuckyStock{{Index: 0, Remaining: 2, PrizeShape: PrizeShape{PrizeKind: 1, CashFen: 1000},
		Children: []GroupBuyLuckyChildStock{{Index: 0, PrizeShape: PrizeShape{PrizeKind: 2, PointsAmount: 50}}}}}
	awards2, err := SettleGroupBuyBatch(bets, 3, 0, slots, children2, lucky2, []byte("gb-seed"))
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}
	if len(gbAwardsOfKind(awards2, GroupBuyAwardKindLucky)) != 2 || len(gbAwardsOfKind(awards2, GroupBuyAwardKindChild)) != 3 {
		t.Fatalf("per-unit lucky children mismatch: %+v", awards2)
	}
}

// TestSettleGroupBuyBatchReplay 同输入两次 → 结果字节一致
func TestSettleGroupBuyBatchReplay(t *testing.T) {
	slots := []GroupBuySlot{gbSlot(1, "谢谢参与", 2), gbSlot(1, "谢谢参与", 2), gbSlot(2, "金币×50", 2), gbSlot(2, "金币×50", 2), gbSlot(3, "限定皮肤", 1)}
	lucky := []GroupBuyLuckyStock{{Index: 0, Remaining: 1, PrizeShape: PrizeShape{PrizeKind: 1, CashFen: 1000}}}
	bets := []GroupBuyBet{{UserID: 11, DrawID: "d1", Count: 1}, {UserID: 22, DrawID: "d2", Count: 2}}
	run := func() string {
		children := []GroupBuyChildStock{{EntryID: 4, ParentID: 3, Remaining: 1, PrizeShape: PrizeShape{PrizeKind: 2, PointsAmount: 100}}}
		awards, err := SettleGroupBuyBatch(bets, 3, 0, slots, children, lucky, []byte("gb-replay"))
		if err != nil {
			t.Fatalf("settle failed: %v", err)
		}
		b, _ := json.Marshal(awards)
		return string(b)
	}
	if first, second := run(), run(); first != second {
		t.Fatalf("non-deterministic settlement:\n%s\n%s", first, second)
	}
}

// TestSettleGroupBuyBatchPerBetLimit limit=2：三人各一注，超量槽分布后无注超过 2（limit 按注不按人）
func TestSettleGroupBuyBatchPerBetLimit(t *testing.T) {
	slots := []GroupBuySlot{gbSlot(1, "谢谢参与", 2), gbSlot(1, "谢谢参与", 2), gbSlot(2, "金币×50", 2), gbSlot(2, "金币×50", 2), gbSlot(3, "限定皮肤", 1)}
	bets := []GroupBuyBet{{UserID: 11, DrawID: "d1", Count: 1}, {UserID: 22, DrawID: "d2", Count: 1}, {UserID: 33, DrawID: "d3", Count: 1}}
	awards, err := SettleGroupBuyBatch(bets, 3, 2, slots, nil, nil, []byte("gb-limit"))
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}
	if len(awards) != 5 {
		t.Fatalf("award count mismatch: %+v", awards)
	}
	perUser := map[int64]int{}
	for _, a := range awards {
		perUser[a.UserID]++
	}
	for u, c := range perUser {
		if c > 2 {
			t.Fatalf("per-bet limit violated: user=%d count=%d", u, c)
		}
	}
}

// TestSettleGroupBuyBatchErrors 参数错误路径
func TestSettleGroupBuyBatchErrors(t *testing.T) {
	// 总注数与 target 不符
	if _, err := SettleGroupBuyBatch([]GroupBuyBet{{UserID: 1, Count: 1}}, 3, 0, nil, nil, nil, []byte("x")); err == nil {
		t.Fatal("bet total mismatch should error")
	}
	// 槽不足
	if _, err := SettleGroupBuyBatch([]GroupBuyBet{{UserID: 1, Count: 3}}, 3, 0, []GroupBuySlot{gbSlot(1, "e", 1)}, nil, nil, []byte("x")); err == nil {
		t.Fatal("insufficient slots should error")
	}
	// limit=1：每注打底 1 后无资格注 → 报错
	slots := []GroupBuySlot{gbSlot(1, "e1", 2), gbSlot(1, "e1", 2), gbSlot(2, "e2", 3), gbSlot(2, "e2", 3), gbSlot(2, "e2", 3)}
	if _, err := SettleGroupBuyBatch([]GroupBuyBet{{UserID: 1, Count: 3}}, 3, 1, slots, nil, nil, []byte("x")); err == nil {
		t.Fatal("exhausted per-bet limit should error")
	}
	// 幸运池空（防御路径：直接调 settleGroupBuyLucky，无下注却有幸运库存）
	lucky := []GroupBuyLuckyStock{{Index: 0, Remaining: 1, PrizeShape: PrizeShape{PrizeKind: 1, CashFen: 1}}}
	if _, err := settleGroupBuyLucky(nil, lucky, []byte("x")); err == nil {
		t.Fatal("empty lucky pool should error")
	}
}
