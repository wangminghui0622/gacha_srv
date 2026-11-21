package memorymatch

import "testing"

func TestOpeningWithRulesUsesHandAndPairSize(t *testing.T) {
	rules := &RuleSet{
		InitialHandSize:      6,
		PairSize:             3,
		ReplenishCount:       2,
		DestinyIncome:        2,
		DoubleDestinyBuff:    2,
		DestinyReplenishCost: 2,
		DestinyDoubleEnabled: true,
	}
	seed := RoundSeed(testEngineSeed(), 9, 1)
	st := OpeningWithRules(seed, 1, nil, 0, 0, 0, rules)
	if len(st.InitialHand) != 6 || st.DrawPosition != 6 {
		t.Fatalf("initial hand=%d drawPos=%d", len(st.InitialHand), st.DrawPosition)
	}
	hand, err := st.Hand()
	if err != nil {
		t.Fatal(err)
	}
	if len(hand) != 6 {
		t.Fatalf("hand=%d", len(hand))
	}
}

func TestGrantInitialDestiny(t *testing.T) {
	st := &RoundState{
		InitialHand: []int32{1, 2, 7, 8, 13, 14, 19, 20, 25},
		Rules: &RuleSet{
			InitialHandSize:      9,
			PairSize:             2,
			ReplenishCount:       1,
			DestinyIncome:        2,
			DoubleDestinyBuff:    1,
			DestinyReplenishCost: 1,
			DestinyDoubleEnabled: true,
		},
	}
	// 初始手牌：图案 1 两张、图案 2 两张。
	// 只统计第一命运 1：每张 +2 → 4；第二命运不参与初始手牌计数。
	st.GrantInitialDestiny(1)
	if st.AvailableDestiny != 4 {
		t.Fatalf("initial destiny=%d want 4", st.AvailableDestiny)
	}
}

func TestApplyMatchWithThreeCards(t *testing.T) {
	st := &RoundState{
		Deck:             NewDeck(),
		InitialHand:      []int32{1, 2, 3, 8, 9, 10},
		SeedCommitment:   "x",
		DrawPosition:     6,
		AvailableDestiny: 2,
		Rules: &RuleSet{
			InitialHandSize:      6,
			PairSize:             3,
			ReplenishCount:       1,
			DestinyIncome:        2,
			DoubleDestinyBuff:    1,
			DestinyReplenishCost: 1,
			DestinyDoubleEnabled: true,
		},
	}
	if err := st.ApplyMatch(27, 1, 1, true, []int32{1, 2, 3}); err != nil {
		t.Fatalf("apply match: %v", err)
	}
	if st.PairCount != 1 {
		t.Fatalf("pair count=%d", st.PairCount)
	}
	// 配对图案 1 与第一命运相同、第二命运也选 1 -> 天命双倍 buff +1；
	// 补入牌堆第 7 张（图案 2）不是命运 -> 不加。初始 2 + 1 = 3。
	if st.AvailableDestiny != 3 {
		t.Fatalf("available destiny=%d, want 3", st.AvailableDestiny)
	}
	hand, err := st.Hand()
	if err != nil {
		t.Fatal(err)
	}
	// 移除 1,2,3 后手牌 3 张，再补 1 张到 4 张。
	if len(hand) != 4 {
		t.Fatalf("hand size=%d", len(hand))
	}
}

func TestApplyDestinyDrawCostAndIncome(t *testing.T) {
	st := &RoundState{
		Deck:             NewDeck(),
		InitialHand:      []int32{1, 7, 13, 19, 25},
		SeedCommitment:   "x",
		DrawPosition:     5,
		AvailableDestiny: 4,
		Rules: &RuleSet{
			InitialHandSize:      6,
			PairSize:             2,
			ReplenishCount:       1,
			DestinyIncome:        3,
			DoubleDestinyBuff:    1,
			DestinyReplenishCost: 2,
			DestinyDoubleEnabled: false,
		},
	}
	if err := st.ApplyDestinyDraw(27, 1, 0, false); err != nil {
		t.Fatalf("destiny draw: %v", err)
	}
	// 消耗 2，抽到牌堆第 6 张（图案 1，是命运）收益 +3。
	if st.AvailableDestiny != 5 || st.DestinyDraws != 1 {
		t.Fatalf("destiny=%d draws=%d", st.AvailableDestiny, st.DestinyDraws)
	}
}

func TestExactDeckTargets(t *testing.T) {
	rules := &RuleSet{InitialHandSize: 9, PairSize: 2, ReplenishCount: 1}
	seed := RoundSeed(testEngineSeed(), 11, 1)
	for target := 0; target <= 4; target++ {
		st := OpeningWithRules(seed, 2, nil, target, target, 64, rules)
		if err := st.AutoResolve(27, 0, 0, false); err != nil {
			t.Fatal(err)
		}
		if st.PairCount != target {
			t.Fatalf("target %d produced %d pairs", target, st.PairCount)
		}
	}
	threeRules := &RuleSet{InitialHandSize: 9, PairSize: 3, ReplenishCount: 1}
	for target := 0; target <= 3; target++ {
		st := OpeningWithRules(seed, 2, nil, target, target, 64, threeRules)
		if err := st.AutoResolve(27, 0, 0, false); err != nil {
			t.Fatal(err)
		}
		if st.PairCount != target {
			t.Fatalf("pair_size=3 target %d produced %d pairs", target, st.PairCount)
		}
	}
}

func TestValidateRuleSet(t *testing.T) {
	if err := ValidateRuleSet(&RuleSet{InitialHandSize: 0, PairSize: 2, ReplenishCount: 1}); err == nil {
		t.Fatal("expect initial hand validation")
	}
	if err := ValidateRuleSet(&RuleSet{InitialHandSize: 9, PairSize: 7, ReplenishCount: 1}); err == nil {
		t.Fatal("expect pair size validation")
	}
	if err := ValidateRuleSet(&RuleSet{InitialHandSize: 9, PairSize: 2, ReplenishCount: 1}); err != nil {
		t.Fatalf("valid rules rejected: %v", err)
	}
}
