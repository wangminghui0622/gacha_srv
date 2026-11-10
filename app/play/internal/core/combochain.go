package core

import (
	"errors"
	"fmt"
	"sort"
)

// M13 连击链（combo_chain）结算纯函数：语义逐条移植 server_v2 biz/combo_chain_settlement.go。
// 随机源 = ComboChainRandom = FullCarRandom（SHA256(seed‖domain‖BE32(index)) 前 8 字节大端）；
// 本仓 seed = 全局事务 gid（requestId），同一 (participants, 配置, seed) 结果确定可回放。
//
// 无 regular 抽（纯共享奖模式，与 server_v2 一致）：每用户每车限 1 票，最后一票触发整车结算：
//   - COMBO_TIER 触发：comboRandom(seed,"combo-trigger",0)%10000 < probability_bps →
//     按 tier_weight 加权选档（"combo-tier"），大奖得主 = users[comboRandom("combo-winner")%len]；
//     全局奖：前 min(concentrate_threshold, len) 槽给大奖得主，其余槽随机给他人（"global-target"）。
//   - 未触发 → 安全上限分布（safeMax=minThreshold-1；minThreshold 取所有档位含耗尽档的
//     concentrate_threshold 最小值）：lucky 用户拿 safeMax/2+rand%(safeMax/2+1)、
//     他人 rand%(safeMax-1)，剩余 3 轮回补兜底——全部全局奖必须消耗完，否则报错。
//   - 连击（跨车持久）：empty_streak 本车无全局奖且非大奖得主 +1（获奖清零），≥ 阈值发
//     EMPTY_STREAK 奖并清零；win_streak 本车得全局奖且非大奖得主 +1（未得清零），≥ 阈值发
//     WIN_STREAK 奖并清零。连击奖无限量（server_v2 不扣库存，quantity 仅展示）。

// 结算奖类型（awards_json 的 kind）
const (
	ComboAwardKindCombo       = "combo"
	ComboAwardKindGlobal      = "global"
	ComboAwardKindEmptyStreak = "empty_streak"
	ComboAwardKindWinStreak   = "win_streak"
)

// 共享奖类型（shared_prizes_json 的 kind）
const (
	ComboSharedKindComboTier   = "COMBO_TIER"
	ComboSharedKindEmptyStreak = "EMPTY_STREAK"
	ComboSharedKindWinStreak   = "WIN_STREAK"
)

// PrizeShape 资产形状（全局奖槽/共享奖/拼团结算奖共用；kind=0 无资产仅展示）
type PrizeShape struct {
	PrizeKind        int64  `json:"prize_kind"` // 1 CASH 2 POINTS 3 COUPON
	CashFen          int64  `json:"cash_fen"`
	PointsAmount     int64  `json:"points_amount"`
	CouponTemplateNo string `json:"coupon_template_no"`
}

// ComboChainGlobalSlot 全局奖槽（global_slots_json 元素；每槽 1 件，JSON 平铺 prize 形状）
type ComboChainGlobalSlot struct {
	Index int `json:"index"`
	PrizeShape
}

// ComboChainSharedPrize 共享奖配置（shared_prizes_json 元素）
type ComboChainSharedPrize struct {
	Kind                 string `json:"kind"` // COMBO_TIER / EMPTY_STREAK / WIN_STREAK
	Quantity             int32  `json:"quantity"`
	TierWeight           int32  `json:"tier_weight"`
	ConcentrateThreshold int32  `json:"concentrate_threshold"`
	StreakThreshold      int32  `json:"streak_threshold"`
	PrizeShape
}

// ComboChainAward 结算奖结果（state.awards_json 元素；ComboIndex 供补偿回补档位库存）
type ComboChainAward struct {
	UserID     int64  `json:"user_id"`
	Kind       string `json:"kind"` // combo / global / empty_streak / win_streak
	ComboIndex int32  `json:"combo_index,omitempty"`
	CarIndex   int32  `json:"car_index,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	PrizeShape
}

// ComboChainStreak 连击计数（combo_chain_streaks 行值）
type ComboChainStreak struct {
	EmptyStreak int32 `json:"empty_streak"`
	WinStreak   int32 `json:"win_streak"`
}

// ComboChainTierStock COMBO_TIER 档位库存（结算输入；remaining 跨车持久）
type ComboChainTierStock struct {
	Index                int32
	Remaining            int32
	TierWeight           int32
	ConcentrateThreshold int32
	PrizeShape
}

// ComboChainRandom 连击链确定性随机数（复用满车同款派生）
func ComboChainRandom(seed []byte, domain string, index uint32) uint64 {
	return FullCarRandom(seed, domain, index)
}

// AssignCarGlobals 全局奖槽分车：index%totalCars+1==carIndex（server_v2 AssignComboChainGlobals 同款）
func AssignCarGlobals(slots []ComboChainGlobalSlot, carIndex, totalCars int) []ComboChainGlobalSlot {
	if carIndex <= 0 || totalCars <= 0 {
		return nil
	}
	out := make([]ComboChainGlobalSlot, 0)
	for _, slot := range slots {
		if slot.Index%totalCars+1 != carIndex {
			continue
		}
		out = append(out, slot)
	}
	return out
}

// BuildComboTierStock 共享奖 → COMBO_TIER 档位库存（remaining 按下标对齐；跳过非 COMBO_TIER 档）
func BuildComboTierStock(prizes []ComboChainSharedPrize, remaining []int32) []ComboChainTierStock {
	tiers := make([]ComboChainTierStock, 0)
	var comboIndex int32
	for _, prize := range prizes {
		if prize.Kind != ComboSharedKindComboTier || prize.TierWeight <= 0 || prize.ConcentrateThreshold <= 0 {
			continue
		}
		stock := prize.Quantity
		if int(comboIndex) < len(remaining) {
			stock = remaining[comboIndex]
		}
		tiers = append(tiers, ComboChainTierStock{
			Index:                comboIndex,
			Remaining:            stock,
			TierWeight:           prize.TierWeight,
			ConcentrateThreshold: prize.ConcentrateThreshold,
			PrizeShape:           prize.PrizeShape,
		})
		comboIndex++
	}
	return tiers
}

// SettleComboCar 整车结算（server_v2 SettleComboChainCar 同款）。
// participants 须非空（调用方按 userId 升序传入——结算语义依赖 users[i] 序，函数内再排一次防误）；
// globals = 本车全局奖槽；tiers = 全 COMBO_TIER 档位（含耗尽档，参与 minThreshold 计算）；
// streaks 为结算前连击（跨车输入）；返回结算奖、连击推进、档位库存推进。
// 连击奖只认 shared 里 kind 匹配的第一条（server_v2 SharedPrizeByKind 同款取首个）。
func SettleComboCar(participants []int64, globals []ComboChainGlobalSlot, tiers []ComboChainTierStock,
	tierTriggerProbabilityBps int32, shared []ComboChainSharedPrize, seed []byte,
	streaks map[int64]ComboChainStreak) (awards []ComboChainAward, streakNext map[int64]ComboChainStreak, tierNext []int32, err error) {
	if len(participants) == 0 {
		return nil, nil, nil, errors.New("combo chain settlement requires participants")
	}
	if len(globals) == 0 {
		return nil, nil, nil, errors.New("combo chain car has no global prizes")
	}
	users := append([]int64(nil), participants...)
	sort.Slice(users, func(i, j int) bool { return users[i] < users[j] })
	tierNext = make([]int32, len(tiers))
	for i, tier := range tiers {
		tierNext[i] = tier.Remaining
	}
	globalCounts := map[int64]int{}
	comboWinners := map[int64]bool{}
	awards = make([]ComboChainAward, 0, len(globals)+len(users))

	// ① COMBO_TIER 触发判定 + 全局奖分布
	triggered, tier, tierErr := maybeTriggerComboTier(tiers, len(globals), tierTriggerProbabilityBps, seed)
	if tierErr != nil {
		return nil, nil, nil, tierErr
	}
	if triggered {
		winner := users[ComboChainRandom(seed, "combo-winner", 0)%uint64(len(users))]
		comboWinners[winner] = true
		awards = append(awards, ComboChainAward{UserID: winner, Kind: ComboAwardKindCombo, ComboIndex: tier.Index, PrizeShape: tier.PrizeShape})
		tierNext[tier.Index]--
		target := int(tier.ConcentrateThreshold)
		if target > len(globals) {
			target = len(globals)
		}
		others := make([]int64, 0, len(users)-1)
		for _, userID := range users {
			if userID != winner {
				others = append(others, userID)
			}
		}
		for index, unit := range globals {
			targetUser := winner
			if index >= target && len(others) > 0 {
				targetUser = others[ComboChainRandom(seed, "global-target", uint32(index))%uint64(len(others))]
			}
			if targetUser != winner {
				globalCounts[targetUser]++
			}
			awards = append(awards, ComboChainAward{UserID: targetUser, Kind: ComboAwardKindGlobal, PrizeShape: unit.PrizeShape})
		}
	} else if err := distributeGlobalsWithoutTrigger(users, globals, tiers, seed, globalCounts, &awards); err != nil {
		return nil, nil, nil, err
	}

	// ② 连击推进（server_v2 三段循环同款：empty 发奖判定 → win 推进/发奖 → empty 推进/清零）
	emptyPrize := sharedPrizeByKind(shared, ComboSharedKindEmptyStreak)
	winPrize := sharedPrizeByKind(shared, ComboSharedKindWinStreak)
	streakNext = copyComboStreaks(streaks, users)
	gotEmptyAward := map[int64]bool{}
	for _, userID := range users {
		wonGlobal := globalCounts[userID] > 0
		if emptyPrize != nil && emptyPrize.StreakThreshold > 0 && !comboWinners[userID] && !wonGlobal {
			state := streakNext[userID]
			if state.EmptyStreak+1 >= emptyPrize.StreakThreshold {
				gotEmptyAward[userID] = true
				awards = append(awards, ComboChainAward{UserID: userID, Kind: ComboAwardKindEmptyStreak, PrizeShape: emptyPrize.PrizeShape})
			}
		}
	}
	for _, userID := range users {
		if comboWinners[userID] {
			continue
		}
		wonGlobal := globalCounts[userID] > 0
		state := streakNext[userID]
		if wonGlobal && winPrize != nil && winPrize.StreakThreshold > 0 {
			state.WinStreak++
			if state.WinStreak >= winPrize.StreakThreshold {
				awards = append(awards, ComboChainAward{UserID: userID, Kind: ComboAwardKindWinStreak, PrizeShape: winPrize.PrizeShape})
				state.WinStreak = 0
			}
		} else if !wonGlobal {
			state.WinStreak = 0
		}
		streakNext[userID] = state
	}
	for _, userID := range users {
		state := streakNext[userID]
		wonGlobal := globalCounts[userID] > 0
		switch {
		case wonGlobal || comboWinners[userID]:
			state.EmptyStreak = 0
		case gotEmptyAward[userID]:
			state.EmptyStreak = 0
		default:
			state.EmptyStreak++
		}
		streakNext[userID] = state
	}
	return awards, streakNext, tierNext, nil
}

// maybeTriggerComboTier 触发判定（server_v2 maybeTriggerComboTier 同款）：
// 有效档 = remaining>0 且 0<concentrate_threshold≤len(globals)；概率万分数判定；加权选档。
func maybeTriggerComboTier(tiers []ComboChainTierStock, globalCount int, probabilityBPS int32, seed []byte) (bool, *ComboChainTierStock, error) {
	valid := make([]ComboChainTierStock, 0, len(tiers))
	var totalWeight uint64
	for _, tier := range tiers {
		if tier.Remaining <= 0 || tier.ConcentrateThreshold <= 0 || int(tier.ConcentrateThreshold) > globalCount {
			continue
		}
		valid = append(valid, tier)
		totalWeight += uint64(tier.TierWeight)
	}
	if totalWeight == 0 || globalCount == 0 {
		return false, nil, nil
	}
	if probabilityBPS == 0 || probabilityBPS > 10000 {
		return false, nil, errors.New("invalid combo tier trigger probability")
	}
	if uint32(ComboChainRandom(seed, "combo-trigger", 0)%10000) >= uint32(probabilityBPS) {
		return false, nil, nil
	}
	pick := ComboChainRandom(seed, "combo-tier", 0) % totalWeight
	var cumulative uint64
	for index := range valid {
		cumulative += uint64(valid[index].TierWeight)
		if pick < cumulative {
			tier := valid[index]
			return true, &tier, nil
		}
	}
	tier := valid[len(valid)-1]
	return true, &tier, nil
}

// distributeGlobalsWithoutTrigger 未触发时的安全上限分布（server_v2 同款）：
// minThreshold = 全档位（含耗尽）concentrate_threshold 最小值；无档位时 = len(globals)/2+1；
// safeMax = minThreshold-1（最小 1）；lucky 用户拿 safeMax/2+rand%(safeMax/2+1)、
// 他人 rand%(safeMax-1)，剩余按 3 轮回补（有奖补到 safeMax → 零奖补 1 → 均分）全部消耗完。
func distributeGlobalsWithoutTrigger(users []int64, globals []ComboChainGlobalSlot, tiers []ComboChainTierStock, seed []byte, globalCounts map[int64]int, awards *[]ComboChainAward) error {
	minThreshold := int32(9999)
	var depletedMin int32 = 9999
	for _, tier := range tiers {
		if tier.Remaining > 0 && tier.ConcentrateThreshold > 0 && tier.ConcentrateThreshold < minThreshold {
			minThreshold = tier.ConcentrateThreshold
		}
		if tier.Remaining == 0 && tier.ConcentrateThreshold > 0 && tier.ConcentrateThreshold < depletedMin {
			depletedMin = tier.ConcentrateThreshold
		}
	}
	if depletedMin < minThreshold {
		minThreshold = depletedMin
	}
	if minThreshold == 9999 {
		minThreshold = int32(len(globals)/2 + 1)
		if minThreshold < 2 {
			minThreshold = 2
		}
	}
	safeMax := int(minThreshold) - 1
	if safeMax < 1 {
		safeMax = 1
	}
	distribution := make([]int, len(users))
	remaining := len(globals)
	luckyUser := int(ComboChainRandom(seed, "non-trigger-lucky", 0) % uint64(len(users)))
	luckyShare := safeMax/2 + int(ComboChainRandom(seed, "non-trigger-lucky-share", 0)%uint64(safeMax/2+1))
	if luckyShare > remaining {
		luckyShare = remaining
	}
	distribution[luckyUser] = luckyShare
	remaining -= luckyShare
	for index := range users {
		if index == luckyUser || remaining == 0 {
			continue
		}
		maxShare := remaining
		if maxShare > safeMax-1 {
			maxShare = safeMax - 1
		}
		if maxShare <= 0 {
			continue
		}
		share := int(ComboChainRandom(seed, "non-trigger-share", uint32(index)) % uint64(maxShare+1))
		distribution[index] = share
		remaining -= share
	}
	for remaining > 0 {
		assigned := false
		for index := range users {
			if distribution[index] > 0 && distribution[index] < safeMax && remaining > 0 {
				distribution[index]++
				remaining--
				assigned = true
			}
		}
		if !assigned {
			for index := range users {
				if distribution[index] == 0 && remaining > 0 {
					distribution[index]++
					remaining--
					assigned = true
				}
			}
		}
		if !assigned {
			for index := range users {
				if remaining == 0 {
					break
				}
				distribution[index]++
				remaining--
			}
		}
	}
	awardIdx := 0
	for userIndex, userID := range users {
		count := distribution[userIndex]
		globalCounts[userID] = count
		for i := 0; i < count && awardIdx < len(globals); i++ {
			unit := globals[awardIdx]
			awardIdx++
			*awards = append(*awards, ComboChainAward{UserID: userID, Kind: ComboAwardKindGlobal, PrizeShape: unit.PrizeShape})
		}
	}
	if awardIdx != len(globals) {
		return fmt.Errorf("combo chain global distribution did not consume all prizes: %d/%d", awardIdx, len(globals))
	}
	return nil
}

// sharedPrizeByKind 取指定类型共享奖（server_v2 SharedPrizeByKind 同款：取首个匹配）
func sharedPrizeByKind(shared []ComboChainSharedPrize, kind string) *ComboChainSharedPrize {
	for i := range shared {
		if shared[i].Kind == kind {
			return &shared[i]
		}
	}
	return nil
}

// copyComboStreaks 连击快照：只保留参与用户（结算只推进本车用户）
func copyComboStreaks(source map[int64]ComboChainStreak, users []int64) map[int64]ComboChainStreak {
	result := make(map[int64]ComboChainStreak, len(users))
	for _, userID := range users {
		if source != nil {
			result[userID] = source[userID]
		}
	}
	return result
}
