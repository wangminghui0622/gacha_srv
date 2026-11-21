// Package memorymatch M15b 记忆翻牌引擎：语义逐条移植自 server_v2 biz/data memory_match。
//
// 牌堆 54 张（id 1..54），9 图案 × 每图案 6 张，pattern=(card-1)/6+1；初始手牌 = 洗牌后前 9 张，
// 手牌恒 ≤9。回合随机源为服务端可审计的 SHA256 计数器流：
//
//	roundSeed = SHA256("memory-match/v1/round" ‖ engineSeed(32B crypto/rand) ‖ BE64(sessionID) ‖ BE32(roundNo))
//	next()     = SHA256(roundSeed ‖ BE64(counter)) 前 8 字节大端，counter 递增；uint64n 拒绝采样
//	洗牌       = Fisher-Yates；seed_commitment = SHA256(roundSeed)
//	EXACT 试发 = SHA256("memory-match/v1/deck-attempt" ‖ roundSeed ‖ BE32(attemptIndex))，
//	             默认 32 次上限 64，取 AutoResolve（无命运模拟）后配对数距 [min,max] 最近的牌组
//	final_hash = SHA256("memory-match/v1/final" ‖ r1.ResultHash ‖ … ‖ rN.ResultHash)
//
// 回合推进：ApplyPair（两卡互异同图案都在手牌 → 移除、pair_count++、命运图案配对/补牌 +available_destiny、
// 堆未空且手牌<9 补一张）、ApplyDestinyDraw（无对子且资源可用 → 抽一张）、Terminal（配对数达标或无对子且
// 命运耗尽/堆空/手牌满）、AutoResolve（有对配对否则命运抽牌循环至 Terminal）。
//
// 常量（不可改，照抄）：rounds∈[1,100]、max_pairs∈[1,27]、pattern 1..9、preset 1..100、face 恰 9 张、
// name≤128、URL≤2048、available_destiny≤81、actions≤108、engine_version=1。
package memorymatch

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ── 常量（照抄 server_v2，不可改）──

const (
	DeckSize             = 54 // 总卡数
	PatternCount         = 9  // 图案数
	CardsPerPattern      = 6  // 每图案张数
	InitialHandSize      = 9  // 初始手牌/手牌上限
	EngineVersion        = 1  // 引擎版本（恒 1）
	MaxAvailableDestiny  = 81 // available_destiny 上限
	MaxActions           = 108
	DefaultExactAttempts = 32
	MaxExactAttempts     = 64

	MaxRoundsPerPurchase = 100
	MaxPairCountLimit    = 27
	MinPresetID          = 1
	MaxPresetID          = 100
	MaxPresetNameLen     = 128
	MaxImageURLLen       = 2048

	domainRound       = "memory-match/v1/round"
	domainDeckAttempt = "memory-match/v1/deck-attempt"
	domainFinal       = "memory-match/v1/final"
)

// ── 引擎哨兵错误（logic 层映射 errorx：3039/3040/3041/3042；入参类 400）──

var (
	ErrRoundFinished      = errors.New("memory_match: round already finished")
	ErrRoundNotFinished   = errors.New("memory_match: round not finished")
	ErrDestinyUnavailable = errors.New("memory_match: destiny draw unavailable")
	ErrInvalidPair        = errors.New("memory_match: invalid pair")
	ErrStateInvalid       = errors.New("memory_match: round state invalid")
	ErrConfigInvalid      = errors.New("memory_match: config invalid")
	ErrOpeningInvalid     = errors.New("memory_match: opening invalid")
	ErrActionsLimit       = errors.New("memory_match: actions limit exceeded")
)

// ── RNG：SHA256 计数器流（服务端可审计）──

// RoundSeed 回合随机种子：SHA256(domainRound ‖ engineSeed ‖ BE64(sessionID) ‖ BE32(roundNo))
func RoundSeed(engineSeed []byte, sessionID, roundNo int64) []byte {
	h := sha256.New()
	h.Write([]byte(domainRound))
	h.Write(engineSeed)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(sessionID))
	h.Write(b[:])
	var r [4]byte
	binary.BigEndian.PutUint32(r[:], uint32(roundNo))
	h.Write(r[:])
	return h.Sum(nil)
}

// CounterRNG SHA256 计数器流：next() = SHA256(seed ‖ BE64(counter)) 前 8 字节大端
type CounterRNG struct {
	seed    []byte
	counter uint64
}

func NewCounterRNG(seed []byte) *CounterRNG {
	return &CounterRNG{seed: append([]byte(nil), seed...)}
}

func (r *CounterRNG) Next() uint64 {
	h := sha256.New()
	h.Write(r.seed)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], r.counter)
	h.Write(b[:])
	r.counter++
	out := h.Sum(nil)
	return binary.BigEndian.Uint64(out[:8])
}

// Uint64n 拒绝采样 [0,n)
func (r *CounterRNG) Uint64n(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	limit := ^uint64(0) - (^uint64(0) % n)
	for {
		v := r.Next()
		if v < limit {
			return v % n
		}
	}
}

// SeedCommitment 回合种子承诺：hex(SHA256(roundSeed))——先亮承诺后出牌
func SeedCommitment(roundSeed []byte) string {
	s := sha256.Sum256(roundSeed)
	return hex.EncodeToString(s[:])
}

// DeckAttemptSeed EXACT 发牌第 idx 次尝试种子
func DeckAttemptSeed(roundSeed []byte, attemptIndex int32) []byte {
	h := sha256.New()
	h.Write([]byte(domainDeckAttempt))
	h.Write(roundSeed)
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(attemptIndex))
	h.Write(b[:])
	return h.Sum(nil)
}

// FinalHash 终局结果哈希：SHA256(domainFinal ‖ r1 ‖ … ‖ rN)
func FinalHash(roundHashes [][]byte) []byte {
	h := sha256.New()
	h.Write([]byte(domainFinal))
	for _, rh := range roundHashes {
		h.Write(rh)
	}
	return h.Sum(nil)
}

// ── 牌堆 ──

// PatternOfCard 卡所属图案：(card-1)/6+1（card 1..54）
func PatternOfCard(card int32) int32 { return (card-1)/CardsPerPattern + 1 }

// NewDeck 顺序牌堆 1..54
func NewDeck() []int32 {
	deck := make([]int32, DeckSize)
	for i := range deck {
		deck[i] = int32(i + 1)
	}
	return deck
}

// ShuffleDeck Fisher-Yates 洗牌（rng 驱动，结果确定可回放）
func ShuffleDeck(rng *CounterRNG) []int32 {
	deck := NewDeck()
	for i := len(deck) - 1; i > 0; i-- {
		j := rng.Uint64n(uint64(i + 1))
		deck[i], deck[j] = deck[j], deck[i]
	}
	return deck
}

// ── 回合状态（state_json 持久化结构；当前手牌由 initial_hand+actions 重放派生，可审计）──

// RuleSet 是对对碰全局玩法的可配置参数。
// 旧回合状态没有 Rules 字段时按历史默认值（9 张手牌 / 2 张成碰 / 补 1 张 / 天命收益 1 / 消耗 1）运行。
type RuleSet struct {
	InitialHandSize      int  `json:"initial_hand_size,omitempty"`
	PairSize             int  `json:"pair_size,omitempty"`
	ReplenishCount       int  `json:"replenish_count,omitempty"`
	DestinyDoubleEnabled bool `json:"destiny_double_enabled,omitempty"`
	DestinyIncome        int  `json:"destiny_income,omitempty"`
	DoubleDestinyBuff    int  `json:"double_destiny_buff,omitempty"`
	DestinyReplenishCost int  `json:"destiny_replenish_cost,omitempty"`
}

type RoundState struct {
	Deck             []int32  `json:"deck"`              // 洗牌后整副牌（54 张）
	InitialHand      []int32  `json:"initial_hand"`      // 洗牌后前 N 张
	SeedCommitment   string   `json:"seed_commitment"`   // hex
	DrawPosition     int      `json:"draw_position"`     // 已抽位置（N..54）
	AvailableDestiny int      `json:"available_destiny"` // 命运资源（≤81）
	DestinyDraws     int      `json:"destiny_draws"`     // 命运抽牌次数
	PairCount        int      `json:"pair_count"`
	Actions          []string `json:"actions"` // pair:a:b[:c...] / replenish:c / destinyDraw:c
	Rules            *RuleSet `json:"rules,omitempty"`
}

func (s *RoundState) handLimit() int {
	if s != nil && s.Rules != nil && s.Rules.InitialHandSize > 0 {
		return s.Rules.InitialHandSize
	}
	return InitialHandSize
}

func (s *RoundState) pairSize() int {
	if s != nil && s.Rules != nil && s.Rules.PairSize >= 2 {
		if s.Rules.PairSize > CardsPerPattern {
			return CardsPerPattern
		}
		return s.Rules.PairSize
	}
	return 2
}

func (s *RoundState) replenishCount() int {
	if s != nil && s.Rules != nil && s.Rules.ReplenishCount >= 0 {
		return s.Rules.ReplenishCount
	}
	return 1
}

func (s *RoundState) destinyIncome() int {
	if s != nil && s.Rules != nil && s.Rules.DestinyIncome >= 0 {
		return s.Rules.DestinyIncome
	}
	return 1
}

func (s *RoundState) doubleDestinyBuff() int {
	if s != nil && s.Rules != nil && s.Rules.DoubleDestinyBuff >= 0 {
		return s.Rules.DoubleDestinyBuff
	}
	return 1
}

func (s *RoundState) destinyDrawCost() int {
	if s != nil && s.Rules != nil && s.Rules.DestinyReplenishCost >= 0 {
		return s.Rules.DestinyReplenishCost
	}
	return 1
}

func (s *RoundState) destinyDoubleEnabled() bool {
	return s != nil && s.Rules != nil && s.Rules.DestinyDoubleEnabled
}

// Opening 创建回合开局（历史默认规则）：DYNAMIC 按 roundSeed 洗牌；EXACT 试发匹配 [min,max]
func Opening(roundSeed []byte, strategy int32, patternWeights []int64, exactMin, exactMax, exactAttemptLimit int) *RoundState {
	return OpeningWithRules(roundSeed, strategy, patternWeights, exactMin, exactMax, exactAttemptLimit, nil)
}

// OpeningWithRules 按全局规则创建回合开局。
func OpeningWithRules(roundSeed []byte, strategy int32, patternWeights []int64, exactMin, exactMax, exactAttemptLimit int, rules *RuleSet) *RoundState {
	var deck []int32
	if strategy == 2 { // EXACT
		deck = exactDeck(roundSeed, exactMin, exactMax, exactAttemptLimit, rules)
	} else {
		// UNSPECIFIED 视为 DYNAMIC（配置校验已保证合法）
		deck = ShuffleDeck(NewCounterRNG(roundSeed))
	}
	handSize := InitialHandSize
	if rules != nil && rules.InitialHandSize > 0 {
		handSize = rules.InitialHandSize
	}
	st := &RoundState{
		Deck:           deck,
		InitialHand:    append([]int32(nil), deck[:handSize]...),
		SeedCommitment: SeedCommitment(roundSeed),
		DrawPosition:   handSize,
		Rules:          rules,
	}
	return st
}

// exactDeck EXACT 发牌：逐次尝试（无命运模拟 AutoResolve），取配对数距 [min,max] 最近的牌组，距离 0 即用
func exactDeck(roundSeed []byte, minPairs, maxPairs, attemptLimit int, rules *RuleSet) []int32 {
	if attemptLimit <= 0 {
		attemptLimit = DefaultExactAttempts
	}
	if attemptLimit > MaxExactAttempts {
		attemptLimit = MaxExactAttempts
	}
	handSize := InitialHandSize
	if rules != nil && rules.InitialHandSize > 0 {
		handSize = rules.InitialHandSize
	}
	// 精确目标且 2 张成碰：直接构造恰好 target 对的开局，比随机试发稳定。
	if minPairs == maxPairs && minPairs > 0 && (rules == nil || rules.PairSize <= 2) {
		if deck, ok := buildExactPairDeck(handSize, minPairs); ok {
			return deck
		}
	}
	// 目标 0 碰：构造前 N 张各不同图案的开局（仅当 handSize<=9 时可行），
	// 这样 AutoResolve 会立即终止在 0 对。
	if minPairs == 0 && maxPairs == 0 && handSize <= PatternCount {
		used := make([]bool, DeckSize+1)
		deck := make([]int32, 0, DeckSize)
		for i := 0; i < handSize; i++ {
			card := int32(i*CardsPerPattern + 1)
			deck = append(deck, card)
			used[card] = true
		}
		for card := int32(1); card <= DeckSize; card++ {
			if !used[card] {
				deck = append(deck, card)
			}
		}
		return deck
	}
	best := NewDeck()
	bestDist := int(^uint(0) >> 1)
	for i := int32(0); i < int32(attemptLimit); i++ {
		deck := ShuffleDeck(NewCounterRNG(DeckAttemptSeed(roundSeed, i)))
		sim := &RoundState{
			Deck:           deck,
			InitialHand:    append([]int32(nil), deck[:handSize]...),
			SeedCommitment: SeedCommitment(DeckAttemptSeed(roundSeed, i)),
			DrawPosition:   handSize,
			Rules:          rules,
		}
		_ = sim.AutoResolve(MaxPairCountLimit, 0, 0, false) // 无命运模拟：纯配对链
		d := distToRange(sim.PairCount, minPairs, maxPairs)
		if d == 0 {
			return deck
		}
		if d < bestDist {
			best, bestDist = deck, d
		}
	}
	return best
}

// buildExactPairDeck 构造恰好 pairs 对（2 张成碰）的开局：
// 手牌 = pairs 对 + 若干单张，补牌依次补入尚未出现的图案，补完后手牌全不同、无新对子。
func buildExactPairDeck(handSize, pairs int) ([]int32, bool) {
	if pairs <= 0 || handSize < 2 || pairs*2 > handSize {
		return nil, false
	}
	singles := handSize - pairs*2
	usedPatterns := pairs + singles
	if usedPatterns > PatternCount || usedPatterns+pairs > PatternCount {
		return nil, false
	}
	cards := make([]int32, 0, DeckSize)
	used := make([]bool, DeckSize+1)
	add := func(card int32) bool {
		if card < 1 || card > DeckSize || used[card] {
			return false
		}
		used[card] = true
		cards = append(cards, card)
		return true
	}
	for p := 1; p <= pairs; p++ {
		base := int32((p-1)*CardsPerPattern + 1)
		if !add(base) || !add(base+1) {
			return nil, false
		}
	}
	for i := 0; i < singles; i++ {
		p := pairs + 1 + i
		if !add(int32((p-1)*CardsPerPattern + 1)) {
			return nil, false
		}
	}
	for i := 0; i < pairs; i++ {
		p := usedPatterns + 1 + i
		if p > PatternCount || !add(int32((p-1)*CardsPerPattern+1)) {
			return nil, false
		}
	}
	for card := int32(1); card <= DeckSize; card++ {
		if !used[card] {
			cards = append(cards, card)
		}
	}
	if len(cards) != DeckSize {
		return nil, false
	}
	return cards, true
}

func distToRange(v, lo, hi int) int {
	if v < lo {
		return lo - v
	}
	if v > hi {
		return v - hi
	}
	return 0
}

// Hand 由 initial_hand+actions 重放派生当前手牌（返回副本；状态非法返回 error）
func (s *RoundState) Hand() ([]int32, error) {
	hand := append([]int32(nil), s.InitialHand...)
	// 起始抽牌位置由当前 draw_position 减去已发生的抽牌动作推导。
	// 这样旧状态/手工构造状态即使 initial_hand 长度与 draw_position 不完全一致也能回放。
	pos := s.DrawPosition
	for _, a := range s.Actions {
		if strings.HasPrefix(a, "replenish:") || strings.HasPrefix(a, "destinyDraw:") {
			pos--
		}
	}
	if pos < 0 {
		return nil, ErrStateInvalid
	}
	for _, a := range s.Actions {
		switch {
		case strings.HasPrefix(a, "pair:"):
			parts := strings.Split(a, ":")
			if len(parts) < 3 {
				return nil, ErrStateInvalid
			}
			for _, part := range parts[1:] {
				card, err := strconv.ParseInt(part, 10, 32)
				if err != nil {
					return nil, ErrStateInvalid
				}
				hand = removeCard(hand, int32(card))
			}
		case strings.HasPrefix(a, "replenish:") || strings.HasPrefix(a, "destinyDraw:"):
			idx := strings.IndexByte(a, ':')
			if _, err := strconv.ParseInt(a[idx+1:], 10, 32); err != nil {
				return nil, ErrStateInvalid
			}
			if pos >= len(s.Deck) {
				return nil, ErrStateInvalid
			}
			hand = append(hand, s.Deck[pos])
			pos++
		default:
			return nil, ErrStateInvalid
		}
	}
	if pos != s.DrawPosition {
		return nil, ErrStateInvalid
	}
	return hand, nil
}

func removeCard(hand []int32, card int32) []int32 {
	for i, c := range hand {
		if c == card {
			return append(hand[:i], hand[i+1:]...)
		}
	}
	return hand
}

// FirstMatch 按当前 pair_size 找手牌首个同图案组合（按图案 1..9 扫）；无则 ok=false
func (s *RoundState) FirstMatch() ([]int32, bool) {
	return s.firstMatchN(s.pairSize())
}

func (s *RoundState) firstMatchN(n int) ([]int32, bool) {
	if n < 2 {
		n = 2
	}
	if n > CardsPerPattern {
		n = CardsPerPattern
	}
	hand, err := s.Hand()
	if err != nil {
		return nil, false
	}
	for p := int32(1); p <= PatternCount; p++ {
		cards := make([]int32, 0, n)
		for _, c := range hand {
			if PatternOfCard(c) == p {
				cards = append(cards, c)
				if len(cards) == n {
					return cards, true
				}
			}
		}
	}
	return nil, false
}

// FirstPair 兼容旧接口：固定按 2 张成碰查找。
func (s *RoundState) FirstPair() (int32, int32, bool) {
	cards, ok := s.firstMatchN(2)
	if !ok || len(cards) < 2 {
		return 0, 0, false
	}
	return cards[0], cards[1], true
}

// IsTerminal 终局判定：配对数达标；或无匹配且（命运耗尽/堆空/手牌满）
func (s *RoundState) IsTerminal(maxPairs int) bool {
	if s.PairCount >= maxPairs {
		return true
	}
	if _, ok := s.FirstMatch(); ok {
		return false
	}
	hand, err := s.Hand()
	if err != nil {
		return true
	}
	return s.AvailableDestiny <= 0 || s.DrawPosition >= DeckSize || len(hand) >= s.handLimit()
}

func (s *RoundState) addDestinyForPattern(pattern, destinyPattern, secondDestinyPattern int32, secondDestinyEnabled bool) {
	if destinyPattern > 0 && pattern == destinyPattern {
		s.AvailableDestiny += s.destinyIncome()
	}
	if secondDestinyEnabled && secondDestinyPattern > 0 && pattern == secondDestinyPattern {
		s.AvailableDestiny++
		if s.destinyDoubleEnabled() {
			s.AvailableDestiny += s.doubleDestinyBuff()
		}
	}
	if s.AvailableDestiny > MaxAvailableDestiny {
		s.AvailableDestiny = MaxAvailableDestiny
	}
}

// ApplyMatch 配成一组：N 张同图案且都在手牌 → 移除、pair_count++；
// 配对图案==命运 → 增加出天命收益；第二命运图案 → +1，天命双倍开启时再叠加 buff；
// 碰后按 replenish_count 补牌，补入命运图案 → 增加出天命收益。
func (s *RoundState) ApplyMatch(maxPairs int, destinyPattern, secondDestinyPattern int32, secondDestinyEnabled bool, cards []int32) error {
	if s.IsTerminal(maxPairs) {
		return ErrRoundFinished
	}
	pairSize := s.pairSize()
	if len(cards) != pairSize {
		return ErrInvalidPair
	}
	seen := make(map[int32]bool, len(cards))
	for _, card := range cards {
		if card < 1 || card > DeckSize || seen[card] || PatternOfCard(card) != PatternOfCard(cards[0]) {
			return ErrInvalidPair
		}
		seen[card] = true
	}
	hand, err := s.Hand()
	if err != nil {
		return err
	}
	for _, card := range cards {
		if !inHand(hand, card) {
			return ErrInvalidPair
		}
	}
	if len(s.Actions)+1 > MaxActions {
		return ErrActionsLimit
	}
	pattern := PatternOfCard(cards[0])
	parts := make([]string, 0, len(cards)+1)
	parts = append(parts, "pair")
	for _, card := range cards {
		parts = append(parts, strconv.FormatInt(int64(card), 10))
	}
	s.Actions = append(s.Actions, strings.Join(parts, ":"))
	s.PairCount++
	s.addDestinyForPattern(pattern, destinyPattern, secondDestinyPattern, secondDestinyEnabled)

	handLimit := s.handLimit()
	for i := 0; i < s.replenishCount(); i++ {
		if s.DrawPosition >= DeckSize || len(hand)-len(cards)+i >= handLimit || len(s.Actions)+1 > MaxActions {
			break
		}
		c := s.Deck[s.DrawPosition]
		s.DrawPosition++
		s.Actions = append(s.Actions, fmt.Sprintf("replenish:%d", c))
		s.addDestinyForPattern(PatternOfCard(c), destinyPattern, secondDestinyPattern, secondDestinyEnabled)
	}
	return nil
}

// ApplyPair 兼容旧 API：固定 2 张成碰；pair_size != 2 时请使用 ApplyMatch。
func (s *RoundState) ApplyPair(maxPairs int, destinyPattern, secondDestinyPattern int32, secondDestinyEnabled bool, firstCard, secondCard int32) error {
	if s.pairSize() != 2 {
		return ErrInvalidPair
	}
	return s.ApplyMatch(maxPairs, destinyPattern, secondDestinyPattern, secondDestinyEnabled, []int32{firstCard, secondCard})
}

// ApplyDestinyDraw 命运抽牌：配对数未满、无匹配、命运资源足够、堆未空、手牌未满 → 抽一张；
// 消耗 destiny_replenish_cost；抽到命运图案 → 增加出天命收益。
func (s *RoundState) ApplyDestinyDraw(maxPairs int, destinyPattern, secondDestinyPattern int32, secondDestinyEnabled bool) error {
	if s.IsTerminal(maxPairs) {
		return ErrRoundFinished
	}
	hand, err := s.Hand()
	if err != nil {
		return err
	}
	if len(hand) >= s.handLimit() {
		return ErrDestinyUnavailable
	}
	if _, ok := s.FirstMatch(); ok {
		return ErrDestinyUnavailable
	}
	cost := s.destinyDrawCost()
	if cost < 0 || s.AvailableDestiny < cost {
		return ErrDestinyUnavailable
	}
	if s.DrawPosition >= DeckSize {
		return ErrDestinyUnavailable
	}
	if len(s.Actions)+1 > MaxActions {
		return ErrActionsLimit
	}
	c := s.Deck[s.DrawPosition]
	s.DrawPosition++
	s.AvailableDestiny -= cost
	s.DestinyDraws++
	s.Actions = append(s.Actions, fmt.Sprintf("destinyDraw:%d", c))
	s.addDestinyForPattern(PatternOfCard(c), destinyPattern, secondDestinyPattern, secondDestinyEnabled)
	return nil
}

// AutoResolve 自动推进（跳过/兜底）：循环「有匹配则配，否则命运抽牌」至 Terminal
func (s *RoundState) AutoResolve(maxPairs int, destinyPattern, secondDestinyPattern int32, secondDestinyEnabled bool) error {
	for !s.IsTerminal(maxPairs) {
		if cards, ok := s.FirstMatch(); ok {
			if err := s.ApplyMatch(maxPairs, destinyPattern, secondDestinyPattern, secondDestinyEnabled, cards); err != nil {
				return err
			}
			continue
		}
		if err := s.ApplyDestinyDraw(maxPairs, destinyPattern, secondDestinyPattern, secondDestinyEnabled); err != nil {
			return err
		}
	}
	return nil
}

// RoundResult 回合结果：result_json = {"pair_count":n}（无 hash 字段）；result_hash = SHA256(result_json)
func RoundResult(pairCount int) ([]byte, []byte) {
	resultJSON, _ := json.Marshal(struct {
		PairCount int `json:"pair_count"`
	}{PairCount: pairCount})
	hash := sha256.Sum256(resultJSON)
	return resultJSON, hash[:]
}

// ValidateOpening 开局校验（手动进度/跳过续玩共用）：整副牌 54 张且 1..54 各一张、
// initial_hand==deck[:9]、承诺匹配、draw_position∈[9,54]、重放手牌≤9、配对数≤max、
// available_destiny≤81、actions≤108、重放一致性（pair_count/draw_position 与 actions 推导一致）
func ValidateOpening(s *RoundState, roundSeed []byte, maxPairs int) error {
	if len(s.Deck) != DeckSize {
		return ErrOpeningInvalid
	}
	seen := make([]bool, DeckSize+1)
	for _, c := range s.Deck {
		if c < 1 || c > DeckSize || seen[c] {
			return ErrOpeningInvalid
		}
		seen[c] = true
	}
	handLimit := s.handLimit()
	if handLimit < 1 || handLimit > DeckSize || len(s.InitialHand) != handLimit {
		return ErrOpeningInvalid
	}
	for i := 0; i < handLimit; i++ {
		if s.InitialHand[i] != s.Deck[i] {
			return ErrOpeningInvalid
		}
	}
	if s.SeedCommitment != SeedCommitment(roundSeed) {
		return ErrOpeningInvalid
	}
	if s.DrawPosition < handLimit || s.DrawPosition > DeckSize {
		return ErrOpeningInvalid
	}
	if s.PairCount < 0 || s.PairCount > maxPairs {
		return ErrOpeningInvalid
	}
	if s.AvailableDestiny < 0 || s.AvailableDestiny > MaxAvailableDestiny {
		return ErrOpeningInvalid
	}
	if len(s.Actions) > MaxActions {
		return ErrOpeningInvalid
	}
	hand, err := s.Hand()
	if err != nil {
		return ErrOpeningInvalid
	}
	if len(hand) > handLimit {
		return ErrOpeningInvalid
	}
	return nil
}

// ── 奖档精确匹配 ──

// EntrySpec 奖档（按 total_pair_count 精确匹配；无匹配 → -1 无奖）
type EntrySpec struct {
	EntryID        int64 `json:"entry_id"`
	TotalPairCount int64 `json:"total_pair_count"`
}

// SelectEntryByTotalPairs 按 total_pair_count 精确匹配奖档；无匹配返回 -1
func SelectEntryByTotalPairs(entries []EntrySpec, total int64) int64 {
	for _, e := range entries {
		if e.TotalPairCount == total {
			return e.EntryID
		}
	}
	return -1
}

// ── 配置校验 ──

// FacePreset 正面预设配置（face_presets_json 元素）
type FacePreset struct {
	PresetID int32      `json:"presetId"`
	Name     string     `json:"name"`
	Faces    []FaceItem `json:"faces"`
}

type FaceItem struct {
	PatternID int32  `json:"patternId"`
	Name      string `json:"name"`
	ImageURL  string `json:"imageUrl"`
}

// BackPreset 背面预设配置（back_presets_json 元素）
type BackPreset struct {
	PresetID               int32  `json:"presetId"`
	Name                   string `json:"name"`
	BackImageURL           string `json:"backImageUrl"`
	FaceBackgroundImageURL string `json:"faceBackgroundImageUrl"`
}

// ValidateConfigRules 玩法配置校验：rounds∈[1,100]、max_pairs∈[1,27]、pattern_weights 恰 9 且 >0、
// EXACT 参数合法（min≤max、attempts≤64）、preset id 1..100 不重复、face 恰 9 张（图案 1..9 不重复）、
// name≤128、URL≤2048、擂主 bps∈(0,10000]（champion_min>0 时）
func ValidateConfigRules(rounds, maxPairs int, secondDestinyEnabled bool, deckStrategy int32,
	patternWeights []int64, exactMin, exactMax, exactAttempts int,
	facePresets []FacePreset, backPresets []BackPreset,
	defaultFacePresetID, defaultBackPresetID, championMin, championBps int) error {
	if rounds < 1 || rounds > MaxRoundsPerPurchase {
		return fmt.Errorf("%w: rounds_per_purchase=%d", ErrConfigInvalid, rounds)
	}
	if maxPairs < 1 || maxPairs > MaxPairCountLimit {
		return fmt.Errorf("%w: max_pair_count=%d", ErrConfigInvalid, maxPairs)
	}
	if deckStrategy < 1 || deckStrategy > 2 {
		return fmt.Errorf("%w: deck_strategy=%d", ErrConfigInvalid, deckStrategy)
	}
	if len(patternWeights) != PatternCount {
		return fmt.Errorf("%w: pattern_weights len=%d", ErrConfigInvalid, len(patternWeights))
	}
	for _, w := range patternWeights {
		if w <= 0 {
			return fmt.Errorf("%w: pattern_weight=%d", ErrConfigInvalid, w)
		}
	}
	if deckStrategy == 2 {
		if exactMin < 0 || exactMax < exactMin || exactMax > MaxPairCountLimit {
			return fmt.Errorf("%w: exact range [%d,%d]", ErrConfigInvalid, exactMin, exactMax)
		}
		if exactAttempts < 0 || exactAttempts > MaxExactAttempts {
			return fmt.Errorf("%w: exact_attempts=%d", ErrConfigInvalid, exactAttempts)
		}
	}
	if err := validateFacePresets(facePresets); err != nil {
		return err
	}
	if err := validateBackPresets(backPresets); err != nil {
		return err
	}
	if !presetExists(facePresets, defaultFacePresetID) || !backPresetExists(backPresets, defaultBackPresetID) {
		return fmt.Errorf("%w: default preset missing", ErrConfigInvalid)
	}
	if championMin < 0 {
		return fmt.Errorf("%w: champion_min=%d", ErrConfigInvalid, championMin)
	}
	if championMin > 0 && (championBps <= 0 || championBps > 10000) {
		return fmt.Errorf("%w: champion_bps=%d", ErrConfigInvalid, championBps)
	}
	return nil
}

// ValidateRuleSet 校验扩展规则（全局玩法规则）。
func ValidateRuleSet(r *RuleSet) error {
	if r == nil {
		return nil
	}
	if r.InitialHandSize < 1 || r.InitialHandSize > DeckSize {
		return fmt.Errorf("%w: initial_hand_size=%d", ErrConfigInvalid, r.InitialHandSize)
	}
	if r.PairSize < 2 || r.PairSize > CardsPerPattern {
		return fmt.Errorf("%w: pair_size=%d", ErrConfigInvalid, r.PairSize)
	}
	if r.ReplenishCount < 0 || r.ReplenishCount > DeckSize {
		return fmt.Errorf("%w: replenish_count=%d", ErrConfigInvalid, r.ReplenishCount)
	}
	if r.DestinyIncome < 0 || r.DestinyIncome > MaxAvailableDestiny {
		return fmt.Errorf("%w: destiny_income=%d", ErrConfigInvalid, r.DestinyIncome)
	}
	if r.DoubleDestinyBuff < 0 || r.DoubleDestinyBuff > MaxAvailableDestiny {
		return fmt.Errorf("%w: double_destiny_buff=%d", ErrConfigInvalid, r.DoubleDestinyBuff)
	}
	if r.DestinyReplenishCost < 0 || r.DestinyReplenishCost > MaxAvailableDestiny {
		return fmt.Errorf("%w: destiny_replenish_cost=%d", ErrConfigInvalid, r.DestinyReplenishCost)
	}
	return nil
}

func validateFacePresets(presets []FacePreset) error {
	if len(presets) == 0 {
		return fmt.Errorf("%w: no face presets", ErrConfigInvalid)
	}
	ids := map[int32]bool{}
	for _, p := range presets {
		if p.PresetID < MinPresetID || p.PresetID > MaxPresetID || ids[p.PresetID] {
			return fmt.Errorf("%w: face preset id=%d", ErrConfigInvalid, p.PresetID)
		}
		ids[p.PresetID] = true
		if len(p.Name) == 0 || len(p.Name) > MaxPresetNameLen {
			return fmt.Errorf("%w: face preset name", ErrConfigInvalid)
		}
		if len(p.Faces) != PatternCount {
			return fmt.Errorf("%w: face preset %d faces=%d", ErrConfigInvalid, p.PresetID, len(p.Faces))
		}
		patterns := map[int32]bool{}
		for _, f := range p.Faces {
			if f.PatternID < 1 || f.PatternID > PatternCount || patterns[f.PatternID] {
				return fmt.Errorf("%w: face pattern=%d", ErrConfigInvalid, f.PatternID)
			}
			patterns[f.PatternID] = true
			if len(f.Name) == 0 || len(f.Name) > MaxPresetNameLen {
				return fmt.Errorf("%w: face name", ErrConfigInvalid)
			}
			if len(f.ImageURL) > MaxImageURLLen {
				return fmt.Errorf("%w: face image url", ErrConfigInvalid)
			}
		}
	}
	return nil
}

func validateBackPresets(presets []BackPreset) error {
	if len(presets) == 0 {
		return fmt.Errorf("%w: no back presets", ErrConfigInvalid)
	}
	ids := map[int32]bool{}
	for _, p := range presets {
		if p.PresetID < MinPresetID || p.PresetID > MaxPresetID || ids[p.PresetID] {
			return fmt.Errorf("%w: back preset id=%d", ErrConfigInvalid, p.PresetID)
		}
		ids[p.PresetID] = true
		if len(p.Name) == 0 || len(p.Name) > MaxPresetNameLen {
			return fmt.Errorf("%w: back preset name", ErrConfigInvalid)
		}
		if len(p.BackImageURL) > MaxImageURLLen || len(p.FaceBackgroundImageURL) > MaxImageURLLen {
			return fmt.Errorf("%w: back image url", ErrConfigInvalid)
		}
	}
	return nil
}

func presetExists(presets []FacePreset, id int) bool {
	for _, p := range presets {
		if int(p.PresetID) == id {
			return true
		}
	}
	return false
}

// PresetExists 正面预设是否存在（外观校验导出封装）
func PresetExists(presets []FacePreset, id int) bool { return presetExists(presets, id) }

func backPresetExists(back []BackPreset, id int) bool {
	for _, p := range back {
		if int(p.PresetID) == id {
			return true
		}
	}
	return false
}

// BackPresetExists 背面预设是否存在（外观校验导出封装）
func BackPresetExists(back []BackPreset, id int) bool { return backPresetExists(back, id) }

func inHand(hand []int32, card int32) bool {
	for _, c := range hand {
		if c == card {
			return true
		}
	}
	return false
}

// ChampionCreditMinor 每次购买计入现任擂主 pending 的分成：price×bps/10000
func ChampionCreditMinor(priceMinor, bps int64) int64 {
	return priceMinor * bps / 10000
}

// SortedEntrySpecs 奖档按 total_pair_count 升序（展示用）
func SortedEntrySpecs(entries []EntrySpec) []EntrySpec {
	out := append([]EntrySpec(nil), entries...)
	sort.Slice(out, func(i, j int) bool { return out[i].TotalPairCount < out[j].TotalPairCount })
	return out
}
