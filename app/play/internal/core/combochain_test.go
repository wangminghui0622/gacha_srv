package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

func comboSlot(index int, kind int64, cash, points int64) ComboChainGlobalSlot {
	return ComboChainGlobalSlot{Index: index, PrizeShape: PrizeShape{PrizeKind: kind, CashFen: cash, PointsAmount: points}}
}

func comboShared(kind string, quantity, tierWeight, threshold, streak int32, prizeKind int64, cash, points int64) ComboChainSharedPrize {
	return ComboChainSharedPrize{
		Kind: kind, Quantity: quantity, TierWeight: tierWeight,
		ConcentrateThreshold: threshold, StreakThreshold: streak,
		PrizeShape: PrizeShape{PrizeKind: prizeKind, CashFen: cash, PointsAmount: points},
	}
}

// comboAwardsOfKind 结算奖按 kind 过滤
func comboAwardsOfKind(awards []ComboChainAward, kind string) []ComboChainAward {
	out := make([]ComboChainAward, 0)
	for _, a := range awards {
		if a.Kind == kind {
			out = append(out, a)
		}
	}
	return out
}

// TestSettleComboCarTrigger 触发路径：100% 触发、单档位、concentrate_threshold=1
// 大奖得主 W 拿 combo + global[0]；global[1] 随机给他人 G；N 空手 → empty_streak 1。
// WIN_STREAK 阈值 1：G 中 global → 立即发 win 奖并清零。
func TestSettleComboCarTrigger(t *testing.T) {
	tiers := BuildComboTierStock([]ComboChainSharedPrize{
		comboShared(ComboSharedKindComboTier, 1, 5, 1, 0, 1, 5000, 0),
	}, []int32{1})
	shared := []ComboChainSharedPrize{
		comboShared(ComboSharedKindEmptyStreak, -1, 0, 0, 2, 2, 0, 100),
		comboShared(ComboSharedKindWinStreak, -1, 0, 0, 1, 1, 300, 0),
	}
	globals := []ComboChainGlobalSlot{comboSlot(0, 1, 500, 0), comboSlot(1, 2, 0, 100)}
	awards, streakNext, tierNext, err := SettleComboCar([]int64{3, 1, 2}, globals, tiers, 10000, shared, []byte("gid-1"), nil)
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}
	if len(tierNext) != 1 || tierNext[0] != 0 {
		t.Fatalf("tier stock not consumed: %v", tierNext)
	}
	combo := comboAwardsOfKind(awards, ComboAwardKindCombo)
	global := comboAwardsOfKind(awards, ComboAwardKindGlobal)
	win := comboAwardsOfKind(awards, ComboAwardKindWinStreak)
	empty := comboAwardsOfKind(awards, ComboAwardKindEmptyStreak)
	if len(combo) != 1 || len(global) != 2 || len(win) != 1 || len(empty) != 0 {
		t.Fatalf("award kinds mismatch: combo=%d global=%d win=%d empty=%d (awards=%+v)", len(combo), len(global), len(win), len(empty), awards)
	}
	winner := combo[0].UserID
	// concentrate_threshold=1：winner 拿 global[0]（500 现金）
	if global[0].UserID != winner || global[0].CashFen != 500 {
		t.Fatalf("winner should get global[0]: global=%+v winner=%d", global, winner)
	}
	// global[1] 得主 G：win_streak 阈值 1 → 已发奖清零；empty 清零
	g := global[1].UserID
	if g == winner {
		t.Fatalf("global[1] must not be the combo winner")
	}
	if win[0].UserID != g {
		t.Fatalf("win award should go to global[1] receiver: win=%+v", win)
	}
	if s := streakNext[g]; s.WinStreak != 0 || s.EmptyStreak != 0 {
		t.Fatalf("global receiver streak should reset: %+v", s)
	}
	if s := streakNext[winner]; s.WinStreak != 0 || s.EmptyStreak != 0 {
		t.Fatalf("combo winner streak should stay reset: %+v", s)
	}
	// 空手用户 N：empty_streak 1（阈值 2 未发奖）
	for _, u := range []int64{1, 2, 3} {
		if u != winner && u != g {
			if s := streakNext[u]; s.EmptyStreak != 1 || s.WinStreak != 0 {
				t.Fatalf("empty-handed user streak mismatch: user=%d streak=%+v", u, s)
			}
		}
	}
}

// TestSettleComboCarNoTriggerDepletedTier 未触发路径：档位耗尽（remaining=0）、concentrate_threshold=1
// → safeMax=1：lucky 用户 0 份、他人 0 份，剩余 2 槽回补轮给 users[0]/users[1] 各 1。
func TestSettleComboCarNoTriggerDepletedTier(t *testing.T) {
	tiers := BuildComboTierStock([]ComboChainSharedPrize{
		comboShared(ComboSharedKindComboTier, 1, 5, 1, 0, 1, 5000, 0),
	}, []int32{0})
	shared := []ComboChainSharedPrize{
		comboShared(ComboSharedKindEmptyStreak, -1, 0, 0, 2, 2, 0, 100),
	}
	globals := []ComboChainGlobalSlot{comboSlot(2, 1, 500, 0), comboSlot(3, 2, 0, 100)}
	awards, streakNext, tierNext, err := SettleComboCar([]int64{3, 1, 2}, globals, tiers, 10000, shared, []byte("gid-2"), nil)
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}
	if len(tierNext) != 1 || tierNext[0] != 0 {
		t.Fatalf("tier stock should stay zero: %v", tierNext)
	}
	global := comboAwardsOfKind(awards, ComboAwardKindGlobal)
	if len(global) != 2 {
		t.Fatalf("all globals must be consumed: %+v", awards)
	}
	// 回补轮按 users 升序：users[0]=1、users[1]=2 各 1 份
	if global[0].UserID != 1 || global[1].UserID != 2 {
		t.Fatalf("round-robin fallback mismatch: %+v", global)
	}
	if s := streakNext[int64(3)]; s.EmptyStreak != 1 {
		t.Fatalf("user 3 should have empty_streak 1: %+v", s)
	}
}

// TestSettleComboCarTriggerConcentrateAll 触发路径：concentrate_threshold == 槽数 → 全部槽给大奖得主
// （server_v2 有效档要求 threshold≤槽数，threshold 超过槽数的档位不参与触发）
func TestSettleComboCarTriggerConcentrateAll(t *testing.T) {
	tiers := BuildComboTierStock([]ComboChainSharedPrize{
		comboShared(ComboSharedKindComboTier, 1, 5, 2, 0, 1, 5000, 0),
	}, []int32{1})
	globals := []ComboChainGlobalSlot{comboSlot(0, 1, 500, 0), comboSlot(1, 2, 0, 100)}
	awards, _, _, err := SettleComboCar([]int64{1, 2, 3}, globals, tiers, 10000, nil, []byte("gid-3"), nil)
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}
	combo := comboAwardsOfKind(awards, ComboAwardKindCombo)
	global := comboAwardsOfKind(awards, ComboAwardKindGlobal)
	if len(combo) != 1 || len(global) != 2 {
		t.Fatalf("award kinds mismatch: %+v", awards)
	}
	if global[0].UserID != combo[0].UserID || global[1].UserID != combo[0].UserID {
		t.Fatalf("all globals should go to combo winner: %+v", global)
	}
}

// TestSettleComboCarReplay 同输入两次 → 结果字节一致（确定性可回放）
func TestSettleComboCarReplay(t *testing.T) {
	tiers := BuildComboTierStock([]ComboChainSharedPrize{
		comboShared(ComboSharedKindComboTier, 1, 5, 1, 0, 1, 5000, 0),
	}, []int32{1})
	shared := []ComboChainSharedPrize{
		comboShared(ComboSharedKindEmptyStreak, -1, 0, 0, 2, 2, 0, 100),
		comboShared(ComboSharedKindWinStreak, -1, 0, 0, 1, 1, 300, 0),
	}
	globals := []ComboChainGlobalSlot{comboSlot(0, 1, 500, 0), comboSlot(1, 2, 0, 100)}
	run := func() string {
		awards, streakNext, tierNext, err := SettleComboCar([]int64{1, 2, 3}, globals, tiers, 10000, shared, []byte("replay-seed"), nil)
		if err != nil {
			t.Fatalf("settle failed: %v", err)
		}
		b, _ := json.Marshal([]interface{}{awards, streakNext, tierNext})
		return string(b)
	}
	first, second := run(), run()
	if first != second {
		t.Fatalf("non-deterministic settlement:\n%s\n%s", first, second)
	}
}

// TestSettleComboCarEmptyStreakAward 空手连击达阈值 → 发奖并清零（跨车输入连击）
func TestSettleComboCarEmptyStreakAward(t *testing.T) {
	// 耗尽档位 → 未触发；2 槽回补轮给 users[0]/users[1]；user 3 空手
	tiers := BuildComboTierStock([]ComboChainSharedPrize{
		comboShared(ComboSharedKindComboTier, 1, 5, 1, 0, 1, 5000, 0),
	}, []int32{0})
	shared := []ComboChainSharedPrize{
		comboShared(ComboSharedKindEmptyStreak, -1, 0, 0, 2, 2, 0, 100),
	}
	globals := []ComboChainGlobalSlot{comboSlot(2, 1, 500, 0), comboSlot(3, 2, 0, 100)}
	streaks := map[int64]ComboChainStreak{3: {EmptyStreak: 1}}
	awards, streakNext, _, err := SettleComboCar([]int64{1, 2, 3}, globals, tiers, 10000, shared, []byte("gid-4"), streaks)
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}
	empty := comboAwardsOfKind(awards, ComboAwardKindEmptyStreak)
	if len(empty) != 1 || empty[0].UserID != 3 || empty[0].PointsAmount != 100 {
		t.Fatalf("empty streak award mismatch: %+v", empty)
	}
	if s := streakNext[int64(3)]; s.EmptyStreak != 0 {
		t.Fatalf("empty streak should reset after award: %+v", s)
	}
}

// TestSettleComboCarErrors 参数错误路径
func TestSettleComboCarErrors(t *testing.T) {
	if _, _, _, err := SettleComboCar(nil, []ComboChainGlobalSlot{comboSlot(0, 1, 1, 0)}, nil, 10000, nil, []byte("x"), nil); err == nil {
		t.Fatal("empty participants should error")
	}
	if _, _, _, err := SettleComboCar([]int64{1}, nil, nil, 10000, nil, []byte("x"), nil); err == nil {
		t.Fatal("empty globals should error")
	}
	// 有效档位 + bps 越界 → 报错（server_v2 同款：0 或 >10000 非法）
	tiers := BuildComboTierStock([]ComboChainSharedPrize{
		comboShared(ComboSharedKindComboTier, 1, 5, 1, 0, 1, 1, 0),
	}, []int32{1})
	globals := []ComboChainGlobalSlot{comboSlot(0, 1, 1, 0)}
	if _, _, _, err := SettleComboCar([]int64{1, 2}, globals, tiers, 10001, nil, []byte("x"), nil); err == nil {
		t.Fatal("probability over 10000 should error")
	}
}

// TestAssignCarGlobals 全局奖槽分车：index%totalCars+1==carIndex
func TestAssignCarGlobals(t *testing.T) {
	slots := []ComboChainGlobalSlot{comboSlot(0, 1, 1, 0), comboSlot(1, 1, 2, 0), comboSlot(2, 1, 3, 0), comboSlot(3, 1, 4, 0)}
	car1 := AssignCarGlobals(slots, 1, 2)
	car2 := AssignCarGlobals(slots, 2, 2)
	if !reflect.DeepEqual([]int{0, 2}, []int{car1[0].Index, car1[1].Index}) {
		t.Fatalf("car1 slots mismatch: %+v", car1)
	}
	if !reflect.DeepEqual([]int{1, 3}, []int{car2[0].Index, car2[1].Index}) {
		t.Fatalf("car2 slots mismatch: %+v", car2)
	}
	if AssignCarGlobals(slots, 0, 2) != nil {
		t.Fatal("carIndex 0 should return nil")
	}
}

// TestBuildComboTierStock 档位构建：跳过非 COMBO_TIER；remaining 按下标对齐
func TestBuildComboTierStock(t *testing.T) {
	prizes := []ComboChainSharedPrize{
		comboShared(ComboSharedKindComboTier, 3, 5, 1, 0, 1, 5000, 0),
		comboShared(ComboSharedKindEmptyStreak, -1, 0, 0, 2, 2, 0, 100),
		comboShared(ComboSharedKindComboTier, 2, 7, 2, 0, 2, 0, 200),
		comboShared(ComboSharedKindWinStreak, -1, 0, 0, 1, 1, 300, 0),
	}
	tiers := BuildComboTierStock(prizes, []int32{1, 2})
	if len(tiers) != 2 {
		t.Fatalf("tier count mismatch: %+v", tiers)
	}
	if tiers[0].Index != 0 || tiers[0].Remaining != 1 || tiers[0].TierWeight != 5 {
		t.Fatalf("tier[0] mismatch: %+v", tiers[0])
	}
	if tiers[1].Index != 1 || tiers[1].Remaining != 2 || tiers[1].TierWeight != 7 {
		t.Fatalf("tier[1] mismatch: %+v", tiers[1])
	}
	// remaining 缺失 → 回退配置 quantity
	tiersFallback := BuildComboTierStock(prizes, nil)
	if tiersFallback[0].Remaining != 3 || tiersFallback[1].Remaining != 2 {
		t.Fatalf("fallback quantity mismatch: %+v", tiersFallback)
	}
}
