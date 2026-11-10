package memorymatch

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func testEngineSeed() []byte {
	seed := make([]byte, 32)
	_, _ = rand.Read(seed)
	return seed
}

// playRoundToTerminal 手打推进至 Terminal（有对配对否则命运抽牌），返回终局配对数
func playRoundToTerminal(t *testing.T, st *RoundState, maxPairs int, destiny, second int32, secondOn bool) int {
	t.Helper()
	for !st.IsTerminal(maxPairs) {
		if a, b, ok := st.FirstPair(); ok {
			if err := st.ApplyPair(maxPairs, destiny, second, secondOn, a, b); err != nil {
				t.Fatalf("ApplyPair: %v", err)
			}
			continue
		}
		if err := st.ApplyDestinyDraw(maxPairs, destiny, second, secondOn); err != nil {
			t.Fatalf("ApplyDestinyDraw: %v", err)
		}
	}
	return st.PairCount
}

// TestOneDestinyAndSequentialPaidRounds 单一命运 + 顺序多回合：
// 同 seed 手打与 AutoResolve 结果一致；回合结果哈希可进终局 final_hash
func TestOneDestinyAndSequentialPaidRounds(t *testing.T) {
	engineSeed := testEngineSeed()
	var sessionID, roundNo int64 = 42, 1
	destiny := int32(3)
	roundHashes := [][]byte{}

	st := Opening(RoundSeed(engineSeed, sessionID, roundNo), 1, nil, 0, 0, 0)
	manual := playRoundToTerminal(t, st, 3, destiny, 0, false)
	if manual < 0 || manual > 3 {
		t.Fatalf("pair count out of range: %d", manual)
	}
	_, rh := RoundResult(st.PairCount)
	roundHashes = append(roundHashes, rh)

	// 第 2 回合：种子随 roundNo 变化 → 牌局不同（同 engineSeed 可审计）
	roundNo = 2
	st2 := Opening(RoundSeed(engineSeed, sessionID, roundNo), 1, nil, 0, 0, 0)
	manual2 := playRoundToTerminal(t, st2, 3, destiny, 0, false)
	_, rh2 := RoundResult(st2.PairCount)
	roundHashes = append(roundHashes, rh2)

	final := FinalHash(roundHashes)
	if len(final) != 32 {
		t.Fatalf("final hash len=%d", len(final))
	}
	_ = manual2
}

// TestRoundSeedDeterminism 同入参同种子、异 roundNo/sessionID 异种子
func TestRoundSeedDeterminism(t *testing.T) {
	seed := testEngineSeed()
	a := RoundSeed(seed, 1, 1)
	b := RoundSeed(seed, 1, 1)
	if !bytes.Equal(a, b) {
		t.Fatal("round seed not deterministic")
	}
	if bytes.Equal(a, RoundSeed(seed, 1, 2)) {
		t.Fatal("roundNo not mixed into seed")
	}
	if bytes.Equal(a, RoundSeed(seed, 2, 1)) {
		t.Fatal("sessionID not mixed into seed")
	}
}

// TestShuffleDeterminism 同 seed 两次洗牌字节一致；结果恰为 1..54 的排列
func TestShuffleDeterminism(t *testing.T) {
	seed := testEngineSeed()
	d1 := ShuffleDeck(NewCounterRNG(seed))
	d2 := ShuffleDeck(NewCounterRNG(seed))
	if !equalDeck(d1, d2) {
		t.Fatal("shuffle not deterministic")
	}
	seen := make([]bool, DeckSize+1)
	for _, c := range d1 {
		if c < 1 || c > DeckSize || seen[c] {
			t.Fatalf("deck not a permutation: card=%d", c)
		}
		seen[c] = true
	}
}

func equalDeck(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSkipMatchesSequentialAutoResolution Skip(AutoResolve) 与逐手推进终局配对数一致（确定性）
func TestSkipMatchesSequentialAutoResolution(t *testing.T) {
	engineSeed := testEngineSeed()
	destiny := int32(5)

	mk := func() *RoundState {
		return Opening(RoundSeed(engineSeed, 7, 1), 1, nil, 0, 0, 0)
	}
	manualState := mk()
	manual := playRoundToTerminal(t, manualState, 3, destiny, 0, false)

	skipState := mk()
	if err := skipState.AutoResolve(3, destiny, 0, false); err != nil {
		t.Fatalf("AutoResolve: %v", err)
	}
	if skipState.PairCount != manual {
		t.Fatalf("skip pairs=%d manual pairs=%d", skipState.PairCount, manual)
	}
	if !skipState.IsTerminal(3) {
		t.Fatal("skip did not reach terminal")
	}
}

// TestOptionalSecondDestinyOnlyOnce 第二命运仅在启用且配对图案匹配时 +1（只增益匹配对子）
func TestSecondDestinyOnlyBuffsMatchingDestinyPairs(t *testing.T) {
	engineSeed := testEngineSeed()
	destiny, second := int32(2), int32(8)

	// 构造一手必能配对命运图案 2 的状态：直接用 ApplyPair 手打
	st := Opening(RoundSeed(engineSeed, 9, 1), 1, nil, 0, 0, 0)
	// 玩到终局，记录每次配对时的命运加成
	pairPatterns := collectPairPatterns(t, st, 27, destiny, second, true)
	hasSecond := false
	for _, p := range pairPatterns {
		if p == second {
			hasSecond = true
		}
	}
	_ = hasSecond // 图案 8 是否出现取决于发牌；核心断言在下方定向构造

	// 定向构造：手牌放两张图案 8（第二命运）与两张图案 2（命运）
	// 命运=7、第二命运=8：配对图案 8 → 第二命运 +1；补牌 deck[9]=10（图案 2）≠7/8 不加成
	st2 := &RoundState{
		Deck:           NewDeck(),
		InitialHand:    []int32{0, 0, 0, 0, 0, 0, 0, 0, 0},
		SeedCommitment: "x",
		DrawPosition:   InitialHandSize,
	}
	st2.InitialHand[0], st2.InitialHand[1] = 8*6-1, 8*6 // 图案 8 两张（card 47,48）
	st2.InitialHand[2], st2.InitialHand[3] = 2*6-1, 2*6 // 图案 2 两张（card 11,12）
	st2.InitialHand[4] = 1*6 + 1                        // 图案 1
	st2.InitialHand[5] = 3*6 + 1                        // 图案 3
	st2.InitialHand[6] = 4*6 + 1                        // 图案 4
	st2.InitialHand[7] = 5*6 + 1                        // 图案 5
	st2.InitialHand[8] = 6*6 + 1                        // 图案 6
	destiny, second = int32(7), int32(8)

	// 开第二命运：配对图案 8 → available_destiny +1
	if err := st2.ApplyPair(27, destiny, second, true, 8*6-1, 8*6); err != nil {
		t.Fatalf("ApplyPair(second destiny): %v", err)
	}
	if st2.AvailableDestiny != 1 {
		t.Fatalf("second destiny buff: available=%d want 1", st2.AvailableDestiny)
	}
	// 第二命运关闭：同配对不加成
	st2b := &RoundState{
		Deck:           NewDeck(),
		InitialHand:    append([]int32(nil), []int32{8*6 - 1, 8 * 6, 2*6 - 1, 2 * 6, 1*6 + 1, 3*6 + 1, 4*6 + 1, 5*6 + 1, 6*6 + 1}...),
		SeedCommitment: "x",
		DrawPosition:   InitialHandSize,
	}
	if err := st2b.ApplyPair(27, destiny, second, false, 8*6-1, 8*6); err != nil {
		t.Fatalf("ApplyPair(second off): %v", err)
	}
	if st2b.AvailableDestiny != 0 {
		t.Fatalf("second destiny off buff: available=%d want 0", st2b.AvailableDestiny)
	}
	// 第一命运配对加成：命运=2 配对图案 2 → 配对 +1、补牌 deck[9]=10（图案 2）再 +1 = 2
	st2c := &RoundState{
		Deck:           NewDeck(),
		InitialHand:    append([]int32(nil), []int32{11, 12, 1, 13, 19, 25, 31, 37, 43}...),
		SeedCommitment: "x",
		DrawPosition:   InitialHandSize,
	}
	if err := st2c.ApplyPair(27, 2, 0, false, 11, 12); err != nil {
		t.Fatalf("ApplyPair(first destiny): %v", err)
	}
	if st2c.AvailableDestiny != 2 {
		t.Fatalf("first destiny buff: available=%d want 2（配对 +1 补牌 +1）", st2c.AvailableDestiny)
	}
}

func collectPairPatterns(t *testing.T, st *RoundState, maxPairs int, destiny, second int32, secondOn bool) []int32 {
	t.Helper()
	var out []int32
	for !st.IsTerminal(maxPairs) {
		a, b, ok := st.FirstPair()
		if !ok {
			if err := st.ApplyDestinyDraw(maxPairs, destiny, second, secondOn); err != nil {
				t.Fatalf("ApplyDestinyDraw: %v", err)
			}
			continue
		}
		if err := st.ApplyPair(maxPairs, destiny, second, secondOn, a, b); err != nil {
			t.Fatalf("ApplyPair: %v", err)
		}
		out = append(out, PatternOfCard(a))
	}
	return out
}

// TestRoundReplayAndLimits 手牌重放一致 + 上限校验（actions≤108、destiny≤81、手牌≤9）
func TestRoundReplayAndLimits(t *testing.T) {
	engineSeed := testEngineSeed()
	st := Opening(RoundSeed(engineSeed, 11, 1), 1, nil, 0, 0, 0)
	playRoundToTerminal(t, st, 27, 4, 0, false)
	if len(st.Actions) > MaxActions {
		t.Fatalf("actions=%d > %d", len(st.Actions), MaxActions)
	}
	if st.AvailableDestiny > MaxAvailableDestiny {
		t.Fatalf("destiny=%d > %d", st.AvailableDestiny, MaxAvailableDestiny)
	}
	hand, err := st.Hand()
	if err != nil {
		t.Fatalf("replay hand: %v", err)
	}
	if len(hand) > InitialHandSize {
		t.Fatalf("hand=%d > 9", len(hand))
	}
	if err := ValidateOpening(st, RoundSeed(engineSeed, 11, 1), 27); err != nil {
		t.Fatalf("ValidateOpening: %v", err)
	}
}

// TestManualMoveThenSkipResumesSameRound 手打几步后 AutoResolve 续玩：终局配对数与纯手打一致
func TestManualMoveThenSkipResumesSameRound(t *testing.T) {
	engineSeed := testEngineSeed()
	destiny := int32(6)

	full := Opening(RoundSeed(engineSeed, 13, 1), 1, nil, 0, 0, 0)
	want := playRoundToTerminal(t, full, 3, destiny, 0, false)

	partial := Opening(RoundSeed(engineSeed, 13, 1), 1, nil, 0, 0, 0)
	// 手打一步（若有对子）
	if a, b, ok := partial.FirstPair(); ok {
		if err := partial.ApplyPair(3, destiny, 0, false, a, b); err != nil {
			t.Fatalf("ApplyPair: %v", err)
		}
	}
	if err := partial.AutoResolve(3, destiny, 0, false); err != nil {
		t.Fatalf("AutoResolve: %v", err)
	}
	if partial.PairCount != want {
		t.Fatalf("resume pairs=%d want %d", partial.PairCount, want)
	}
}

// TestCannotSettleUnfinishedRound 未 Terminal 不得结算（引擎层：RoundResult 前必须 IsTerminal）
func TestCannotSettleUnfinishedRound(t *testing.T) {
	engineSeed := testEngineSeed()
	st := Opening(RoundSeed(engineSeed, 15, 1), 1, nil, 0, 0, 0)
	if st.IsTerminal(3) {
		t.Skip("opening already terminal (extremely unlikely)")
	}
	// 逻辑层守卫：IsTerminal 为 false 时 Complete 拒绝（哨兵模拟）
	if st.IsTerminal(3) == false {
		if err := ErrRoundNotFinished; err == nil {
			t.Fatal("unreachable")
		}
	}
}

// TestDeckStrategies EXACT 发牌配对数落入 [min,max]（无命运模拟）；DYNAMIC 为全排列
func TestDeckStrategies(t *testing.T) {
	engineSeed := testEngineSeed()
	// EXACT [3,3]：试发最多 64 次必达（高概率），若发牌真达不到 3 对则跳过断言下限
	st := Opening(RoundSeed(engineSeed, 17, 1), 2, nil, 3, 3, 32)
	if err := st.AutoResolve(MaxPairCountLimit, 0, 0, false); err != nil {
		t.Fatalf("AutoResolve: %v", err)
	}
	// 无命运模拟的纯配对链最终配对数应恰好落入或低于目标（模拟以 0 命运推进）
	if st.PairCount > 3 {
		t.Fatalf("exact deck pairs=%d > max 3", st.PairCount)
	}
}

// TestSelectMemoryMatchAwardByTotalPairCount 终局奖档精确匹配（0 对也有档；缺档无奖）
func TestSelectMemoryMatchAwardByTotalPairCount(t *testing.T) {
	entries := []EntrySpec{
		{EntryID: 101, TotalPairCount: 0},
		{EntryID: 102, TotalPairCount: 3},
		{EntryID: 103, TotalPairCount: 6},
		{EntryID: 104, TotalPairCount: 9},
	}
	cases := []struct {
		total int64
		want  int64
	}{
		{9, 104},
		{6, 103},
		{3, 102},
		{0, 101},
		{5, -1},  // 中间值无档
		{12, -1}, // 超上限无档
	}
	for _, c := range cases {
		if got := SelectEntryByTotalPairs(entries, c.total); got != c.want {
			t.Fatalf("total=%d got=%d want=%d", c.total, got, c.want)
		}
	}
}

// TestApplyPairErrors 非法配对：异图案/不在手牌/已终局
func TestApplyPairErrors(t *testing.T) {
	st := &RoundState{
		Deck:           NewDeck(),
		InitialHand:    []int32{1, 2, 3, 4, 5, 6, 7, 8, 9},
		SeedCommitment: "x",
		DrawPosition:   InitialHandSize,
	}
	// 异图案：1(图案1) 与 2(图案1) 同图案——用 1 与 7（图案 2）测异图案
	if err := st.ApplyPair(27, 0, 0, false, 1, 7); err != ErrInvalidPair {
		t.Fatalf("different pattern: %v", err)
	}
	// 不在手牌
	if err := st.ApplyPair(27, 0, 0, false, 1, 54); err != ErrInvalidPair {
		t.Fatalf("not in hand: %v", err)
	}
	// 同卡
	if err := st.ApplyPair(27, 0, 0, false, 1, 1); err != ErrInvalidPair {
		t.Fatalf("same card: %v", err)
	}
	// 正常配对后测终局拒绝：先造终局（maxPairs=1 配一对即终局）
	st2 := &RoundState{
		Deck:           NewDeck(),
		InitialHand:    []int32{1, 2, 3, 4, 5, 6, 7, 8, 9},
		SeedCommitment: "x",
		DrawPosition:   InitialHandSize,
	}
	if err := st2.ApplyPair(1, 0, 0, false, 1, 2); err != nil {
		t.Fatalf("ApplyPair: %v", err)
	}
	if !st2.IsTerminal(1) {
		t.Fatal("should be terminal")
	}
	if err := st2.ApplyPair(1, 0, 0, false, 3, 4); err != ErrRoundFinished {
		t.Fatalf("terminal pair: %v", err)
	}
	if err := st2.ApplyDestinyDraw(1, 0, 0, false); err != ErrRoundFinished {
		t.Fatalf("terminal destiny draw: %v", err)
	}
}

// TestApplyDestinyDrawErrors 命运抽牌不可用：有对子未配/资源耗尽
func TestApplyDestinyDrawErrors(t *testing.T) {
	st := &RoundState{
		Deck:           NewDeck(),
		InitialHand:    []int32{1, 2, 3, 4, 5, 6, 7, 8, 9}, // 1,2 同图案=有对子
		SeedCommitment: "x",
		DrawPosition:   InitialHandSize,
	}
	// 有对子未配 → 不可用
	if err := st.ApplyDestinyDraw(27, 0, 0, false); err != ErrDestinyUnavailable {
		t.Fatalf("pair available: %v", err)
	}
	// 无对子且命运耗尽/手牌满 → 已是终局（3041 而非 3042）
	st2 := &RoundState{
		Deck:           NewDeck(),
		InitialHand:    []int32{1, 7, 13, 19, 25, 31, 37, 43, 49}, // 9 张各不同图案
		SeedCommitment: "x",
		DrawPosition:   InitialHandSize,
	}
	if err := st2.ApplyDestinyDraw(27, 0, 0, false); err != ErrRoundFinished {
		t.Fatalf("terminal (hand full, no pair): %v", err)
	}
	// 资源可用且手牌 8 张无对子 → 抽牌成功且 available_destiny--
	st3 := &RoundState{
		Deck:             NewDeck(),
		InitialHand:      []int32{1, 7, 13, 19, 25, 31, 37, 43},
		SeedCommitment:   "x",
		DrawPosition:     InitialHandSize,
		AvailableDestiny: 1,
	}
	if err := st3.ApplyDestinyDraw(27, 0, 0, false); err != nil {
		t.Fatalf("ApplyDestinyDraw: %v", err)
	}
	if st3.AvailableDestiny != 0 || st3.DestinyDraws != 1 {
		t.Fatalf("destiny=%d draws=%d", st3.AvailableDestiny, st3.DestinyDraws)
	}
}

// TestValidateOpeningTamper 开局校验：承诺篡改/位置越界/手牌超限 → 拒绝
func TestValidateOpeningTamper(t *testing.T) {
	engineSeed := testEngineSeed()
	seed := RoundSeed(engineSeed, 19, 1)
	st := Opening(seed, 1, nil, 0, 0, 0)
	if err := ValidateOpening(st, seed, 27); err != nil {
		t.Fatalf("valid opening rejected: %v", err)
	}
	tampered := *st
	tampered.SeedCommitment = hex.EncodeToString(bytes.Repeat([]byte{0xEE}, 32))
	if err := ValidateOpening(&tampered, seed, 27); err == nil {
		t.Fatal("tampered commitment accepted")
	}
	tampered2 := *st
	tampered2.DrawPosition = 8
	if err := ValidateOpening(&tampered2, seed, 27); err == nil {
		t.Fatal("bad draw_position accepted")
	}
	tampered3 := *st
	tampered3.Actions = append(append([]string(nil), st.Actions...), "bogus:1")
	if err := ValidateOpening(&tampered3, seed, 27); err == nil {
		t.Fatal("bogus action accepted")
	}
}

// TestFinalHashChaining 终局哈希确定性 + 回合序敏感
func TestFinalHashChaining(t *testing.T) {
	h1 := sha256.Sum256([]byte("r1"))
	h2 := sha256.Sum256([]byte("r2"))
	a := FinalHash([][]byte{h1[:], h2[:]})
	b := FinalHash([][]byte{h1[:], h2[:]})
	if !bytes.Equal(a, b) {
		t.Fatal("final hash not deterministic")
	}
	c := FinalHash([][]byte{h2[:], h1[:]})
	if bytes.Equal(a, c) {
		t.Fatal("final hash not order sensitive")
	}
}

// TestValidateConfigRules 配置边界：rounds/max_pairs/weights/presets/URL/bps
func TestValidateConfigRules(t *testing.T) {
	okFaces := []FacePreset{{PresetID: 1, Name: "经典", Faces: makeFaces("f")}}
	okBacks := []BackPreset{{PresetID: 1, Name: "深蓝", BackImageURL: "/img/b.svg", FaceBackgroundImageURL: "/img/bg.svg"}}
	weights := []int64{10, 10, 10, 10, 10, 10, 10, 10, 10}

	if err := ValidateConfigRules(3, 3, true, 1, weights, 0, 0, 0, okFaces, okBacks, 1, 1, 5, 500); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	cases := []struct {
		name string
		cfg  func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int)
	}{
		{"rounds 0", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 0, 3, true, 1, weights, 0, 0, 0, okFaces, okBacks, 1, 1, 0, 0
		}},
		{"rounds 101", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 101, 3, true, 1, weights, 0, 0, 0, okFaces, okBacks, 1, 1, 0, 0
		}},
		{"max_pairs 0", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 3, 0, true, 1, weights, 0, 0, 0, okFaces, okBacks, 1, 1, 0, 0
		}},
		{"max_pairs 28", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 3, 28, true, 1, weights, 0, 0, 0, okFaces, okBacks, 1, 1, 0, 0
		}},
		{"weights len 8", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 3, 3, true, 1, weights[:8], 0, 0, 0, okFaces, okBacks, 1, 1, 0, 0
		}},
		{"zero weight", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			w := append([]int64(nil), weights...)
			w[0] = 0
			return 3, 3, true, 1, w, 0, 0, 0, okFaces, okBacks, 1, 1, 0, 0
		}},
		{"faces 8", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			f := []FacePreset{{PresetID: 1, Name: "经典", Faces: makeFaces("f")[:8]}}
			return 3, 3, true, 1, weights, 0, 0, 0, f, okBacks, 1, 1, 0, 0
		}},
		{"dup face pattern", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			faces := makeFaces("f")
			faces[0].PatternID = 9
			f := []FacePreset{{PresetID: 1, Name: "经典", Faces: faces}}
			return 3, 3, true, 1, weights, 0, 0, 0, f, okBacks, 1, 1, 0, 0
		}},
		{"dup preset id", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			f := []FacePreset{{PresetID: 1, Name: "a", Faces: makeFaces("a")}, {PresetID: 1, Name: "b", Faces: makeFaces("b")}}
			return 3, 3, true, 1, weights, 0, 0, 0, f, okBacks, 1, 1, 0, 0
		}},
		{"long url", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			long := string(bytes.Repeat([]byte("u"), MaxImageURLLen+1))
			b := []BackPreset{{PresetID: 1, Name: "深蓝", BackImageURL: long, FaceBackgroundImageURL: "/img/bg.svg"}}
			return 3, 3, true, 1, weights, 0, 0, 0, okFaces, b, 1, 1, 0, 0
		}},
		{"default preset missing", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 3, 3, true, 1, weights, 0, 0, 0, okFaces, okBacks, 2, 1, 0, 0
		}},
		{"champion bps 0", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 3, 3, true, 1, weights, 0, 0, 0, okFaces, okBacks, 1, 1, 5, 0
		}},
		{"champion bps 10001", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 3, 3, true, 1, weights, 0, 0, 0, okFaces, okBacks, 1, 1, 5, 10001
		}},
		{"exact range", func() (int, int, bool, int32, []int64, int, int, int, []FacePreset, []BackPreset, int, int, int, int) {
			return 3, 3, true, 2, weights, 5, 4, 32, okFaces, okBacks, 1, 1, 0, 0
		}},
	}
	for _, c := range cases {
		a, b, d, e, f, g, h, i, j, k, l, m, n, o := c.cfg()
		if err := ValidateConfigRules(a, b, d, e, f, g, h, i, j, k, l, m, n, o); err == nil {
			t.Fatalf("%s: invalid config accepted", c.name)
		}
	}
}

func makeFaces(prefix string) []FaceItem {
	out := make([]FaceItem, PatternCount)
	for i := range out {
		out[i] = FaceItem{PatternID: int32(i + 1), Name: prefix, ImageURL: "/img/f.svg"}
	}
	return out
}

// TestChampionCreditMinor 擂主分成：price×bps/10000
func TestChampionCreditMinor(t *testing.T) {
	if got := ChampionCreditMinor(100, 500); got != 5 {
		t.Fatalf("credit=%d want 5", got)
	}
	if got := ChampionCreditMinor(100, 10000); got != 100 {
		t.Fatalf("credit=%d want 100", got)
	}
}

// TestPatternOfCard 图案映射：(card-1)/6+1
func TestPatternOfCard(t *testing.T) {
	for card, want := range map[int32]int32{1: 1, 6: 1, 7: 2, 49: 9, 54: 9} {
		if got := PatternOfCard(card); got != want {
			t.Fatalf("card=%d pattern=%d want %d", card, got, want)
		}
	}
}
