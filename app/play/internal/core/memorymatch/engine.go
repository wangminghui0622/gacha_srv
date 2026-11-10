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

type RoundState struct {
	Deck             []int32  `json:"deck"`              // 洗牌后整副牌（54 张）
	InitialHand      []int32  `json:"initial_hand"`      // 洗牌后前 9 张
	SeedCommitment   string   `json:"seed_commitment"`   // hex
	DrawPosition     int      `json:"draw_position"`     // 已抽位置（9..54）
	AvailableDestiny int      `json:"available_destiny"` // 命运资源（≤81）
	DestinyDraws     int      `json:"destiny_draws"`     // 命运抽牌次数
	PairCount        int      `json:"pair_count"`
	Actions          []string `json:"actions"` // pair:a:b / replenish:c / destinyDraw:c
}

// Opening 创建回合开局（发牌）：DYNAMIC 按 roundSeed 洗牌；EXACT 试发匹配 [min,max]
func Opening(roundSeed []byte, strategy int32, patternWeights []int64, exactMin, exactMax, exactAttemptLimit int) *RoundState {
	var deck []int32
	if strategy == 2 { // EXACT
		deck = exactDeck(roundSeed, exactMin, exactMax, exactAttemptLimit)
	} else {
		// UNSPECIFIED 视为 DYNAMIC（配置校验已保证合法）
		deck = ShuffleDeck(NewCounterRNG(roundSeed))
	}
	st := &RoundState{
		Deck:           deck,
		InitialHand:    append([]int32(nil), deck[:InitialHandSize]...),
		SeedCommitment: SeedCommitment(roundSeed),
		DrawPosition:   InitialHandSize,
	}
	return st
}

// exactDeck EXACT 发牌：逐次尝试（无命运模拟 AutoResolve），取配对数距 [min,max] 最近的牌组，距离 0 即用
func exactDeck(roundSeed []byte, minPairs, maxPairs, attemptLimit int) []int32 {
	if attemptLimit <= 0 {
		attemptLimit = DefaultExactAttempts
	}
	if attemptLimit > MaxExactAttempts {
		attemptLimit = MaxExactAttempts
	}
	best := NewDeck()
	bestDist := int(^uint(0) >> 1)
	for i := int32(0); i < int32(attemptLimit); i++ {
		deck := ShuffleDeck(NewCounterRNG(DeckAttemptSeed(roundSeed, i)))
		sim := &RoundState{
			Deck:           deck,
			InitialHand:    append([]int32(nil), deck[:InitialHandSize]...),
			SeedCommitment: SeedCommitment(DeckAttemptSeed(roundSeed, i)),
			DrawPosition:   InitialHandSize,
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
	pos := InitialHandSize
	for _, a := range s.Actions {
		switch {
		case strings.HasPrefix(a, "pair:"):
			parts := strings.SplitN(a, ":", 3)
			if len(parts) != 3 {
				return nil, ErrStateInvalid
			}
			x, err1 := strconv.ParseInt(parts[1], 10, 32)
			y, err2 := strconv.ParseInt(parts[2], 10, 32)
			if err1 != nil || err2 != nil {
				return nil, ErrStateInvalid
			}
			hand = removeCard(hand, int32(x))
			hand = removeCard(hand, int32(y))
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

// FirstPair 手牌首个同图案对子（按图案 1..9 扫）；无则 ok=false
func (s *RoundState) FirstPair() (int32, int32, bool) {
	hand, err := s.Hand()
	if err != nil {
		return 0, 0, false
	}
	for p := int32(1); p <= PatternCount; p++ {
		var cards []int32
		for _, c := range hand {
			if PatternOfCard(c) == p {
				cards = append(cards, c)
				if len(cards) == 2 {
					return cards[0], cards[1], true
				}
			}
		}
	}
	return 0, 0, false
}

// IsTerminal 终局判定：配对数达标；或无对子且（命运耗尽/堆空/手牌满）
func (s *RoundState) IsTerminal(maxPairs int) bool {
	if s.PairCount >= maxPairs {
		return true
	}
	_, _, ok := s.FirstPair()
	if ok {
		return false
	}
	hand, err := s.Hand()
	if err != nil {
		return true
	}
	return s.AvailableDestiny <= 0 || s.DrawPosition >= DeckSize || len(hand) >= InitialHandSize
}

// ApplyPair 配对：两卡 1..54、互异、同图案、都在手牌 → 移除两张、pair_count++；
// 配对图案==命运 → available_destiny++（第二命运启用且配对图案==第二命运再 +1——第二命运只增益匹配对子）；
// draw_position<54 且手牌<9 → 补一张（补入命运图案 → available_destiny++）。
func (s *RoundState) ApplyPair(maxPairs int, destinyPattern, secondDestinyPattern int32, secondDestinyEnabled bool, firstCard, secondCard int32) error {
	if s.IsTerminal(maxPairs) {
		return ErrRoundFinished
	}
	if firstCard < 1 || firstCard > DeckSize || secondCard < 1 || secondCard > DeckSize ||
		firstCard == secondCard || PatternOfCard(firstCard) != PatternOfCard(secondCard) {
		return ErrInvalidPair
	}
	hand, err := s.Hand()
	if err != nil {
		return err
	}
	if !inHand(hand, firstCard) || !inHand(hand, secondCard) {
		return ErrInvalidPair
	}
	if len(s.Actions)+2 > MaxActions {
		return ErrActionsLimit
	}
	pattern := PatternOfCard(firstCard)
	s.Actions = append(s.Actions, fmt.Sprintf("pair:%d:%d", firstCard, secondCard))
	s.PairCount++
	if destinyPattern > 0 && pattern == destinyPattern {
		s.AvailableDestiny++
	}
	if secondDestinyEnabled && secondDestinyPattern > 0 && pattern == secondDestinyPattern {
		s.AvailableDestiny++
	}
	if s.DrawPosition < DeckSize && len(hand)-2 < InitialHandSize {
		c := s.Deck[s.DrawPosition]
		s.DrawPosition++
		s.Actions = append(s.Actions, fmt.Sprintf("replenish:%d", c))
		if destinyPattern > 0 && PatternOfCard(c) == destinyPattern {
			s.AvailableDestiny++
		}
	}
	return nil
}

// ApplyDestinyDraw 命运抽牌：配对数未满、无对子未配、available_destiny>0、堆未空、手牌<9 → 抽一张；
// available_destiny--、destiny_draws++；抽到命运图案 → available_destiny++。
func (s *RoundState) ApplyDestinyDraw(maxPairs int, destinyPattern, secondDestinyPattern int32, secondDestinyEnabled bool) error {
	if s.IsTerminal(maxPairs) {
		return ErrRoundFinished
	}
	if s.PairCount >= maxPairs {
		return ErrDestinyUnavailable
	}
	hand, err := s.Hand()
	if err != nil {
		return err
	}
	if len(hand) >= InitialHandSize {
		return ErrDestinyUnavailable
	}
	if _, _, ok := s.FirstPair(); ok {
		return ErrDestinyUnavailable
	}
	if s.AvailableDestiny <= 0 {
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
	s.AvailableDestiny--
	s.DestinyDraws++
	s.Actions = append(s.Actions, fmt.Sprintf("destinyDraw:%d", c))
	if destinyPattern > 0 && PatternOfCard(c) == destinyPattern {
		s.AvailableDestiny++
	}
	return nil
}

// AutoResolve 自动推进（跳过/兜底）：循环「有对配对，否则命运抽牌」至 Terminal
func (s *RoundState) AutoResolve(maxPairs int, destinyPattern, secondDestinyPattern int32, secondDestinyEnabled bool) error {
	for !s.IsTerminal(maxPairs) {
		if a, b, ok := s.FirstPair(); ok {
			if err := s.ApplyPair(maxPairs, destinyPattern, secondDestinyPattern, secondDestinyEnabled, a, b); err != nil {
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
	if len(s.InitialHand) != InitialHandSize {
		return ErrOpeningInvalid
	}
	for i := 0; i < InitialHandSize; i++ {
		if s.InitialHand[i] != s.Deck[i] {
			return ErrOpeningInvalid
		}
	}
	if s.SeedCommitment != SeedCommitment(roundSeed) {
		return ErrOpeningInvalid
	}
	if s.DrawPosition < InitialHandSize || s.DrawPosition > DeckSize {
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
	if len(hand) > InitialHandSize {
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
