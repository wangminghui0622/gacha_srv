package core

import (
	"crypto/sha256"
	"encoding/binary"
)

// M12b 满车（full_car）整车结算：语义移植自 server_v2 biz/full_car.go（settleFullCar）。
// 随机源 = SHA256(seed‖domain‖BE32(index)) 前 8 字节大端（server_v2 full_car.go:252-260 同款），
// 同一 (seats, prizes, seed) 结果确定，可回放核对；本仓 seed = 全局事务 gid（requestId）。

// 整车奖层级（server_v2 FullCarPrizeTier 同款）
const (
	FullCarTierLucky  int32 = 1 // 幸运奖（floor(seats/2) 个座位）
	FullCarTierNormal int32 = 2 // 普通奖（其余座位）
)

// FullCarSettlementPrize 整车结算奖配置（full_car_configs.settlement_prizes_json 数组元素）
type FullCarSettlementPrize struct {
	Tier             int32  `json:"tier"`               // 1 LUCKY 2 NORMAL
	Weight           uint64 `json:"weight"`             // 层内权重（>0 参与，0 过滤）
	PrizeKind        int64  `json:"prize_kind"`         // 1 CASH 2 POINTS 3 COUPON
	CashFen          int64  `json:"cash_fen"`           // 现金金额（分）
	PointsAmount     int64  `json:"points_amount"`      // 积分数
	CouponTemplateNo string `json:"coupon_template_no"` // 券模板单号
}

// FullCarAward 整车开奖结果（座位 → 奖）
type FullCarAward struct {
	SeatIndex int32
	UserID    int64
	Prize     FullCarSettlementPrize
	Tier      int32 // 该座位分到的层级（1 LUCKY 2 NORMAL）
}

// FullCarRandom 确定性随机数：SHA256(seed‖domain‖BE32(index)) 前 8 字节大端
func FullCarRandom(seed []byte, domain string, index uint32) uint64 {
	h := sha256.New()
	h.Write(seed)
	h.Write([]byte(domain))
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], index)
	h.Write(encoded[:])
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}

// filterFullCarPrizes 按层级过滤有效奖（weight>0；server_v2 还要求 sku 非零，本仓无 sku 概念）
func filterFullCarPrizes(prizes []FullCarSettlementPrize, tier int32) []FullCarSettlementPrize {
	result := make([]FullCarSettlementPrize, 0, len(prizes))
	for _, p := range prizes {
		if p.Weight != 0 && p.Tier == tier {
			result = append(result, p)
		}
	}
	return result
}

// weightedFullCarPrize 层内加权选奖：selected = random % total，累计扣减（server_v2 同款）
func weightedFullCarPrize(prizes []FullCarSettlementPrize, random uint64) (FullCarSettlementPrize, bool) {
	var total uint64
	for _, p := range prizes {
		total += p.Weight
	}
	if total == 0 {
		return FullCarSettlementPrize{}, false
	}
	selected := random % total
	for _, p := range prizes {
		if selected < p.Weight {
			return p, true
		}
		selected -= p.Weight
	}
	return FullCarSettlementPrize{}, false
}

// FullCarLuckyCount 幸运奖座数：floor(seats/2)；无 lucky 奖 → 0；无 normal 奖 → 全部座位
func FullCarLuckyCount(seatCount, luckyCount, normalCount int) int {
	n := seatCount / 2
	if luckyCount == 0 {
		n = 0
	}
	if normalCount == 0 {
		n = seatCount
	}
	return n
}

// SettleFullCar 确定性整车结算（server_v2 settleFullCar 同款算法）：
// ① 层级序列 = 前 luckyCount 个 LUCKY + 其余 NORMAL；
// ② Fisher-Yates 洗牌（从尾到头，other = FullCarRandom(seed,"tier",i) % (i+1)）；
// ③ 逐座位按层级加权选奖（FullCarRandom(seed,"prize",seatIndex)）。
// seats 长度 = 座数，元素 = 座位用户（整车结算时必已坐满，无 0 值）。
func SettleFullCar(seats []int64, prizes []FullCarSettlementPrize, seed []byte) []FullCarAward {
	lucky := filterFullCarPrizes(prizes, FullCarTierLucky)
	normal := filterFullCarPrizes(prizes, FullCarTierNormal)
	luckyCount := FullCarLuckyCount(len(seats), len(lucky), len(normal))
	tiers := make([]int32, len(seats))
	for i := range tiers {
		if i < luckyCount {
			tiers[i] = FullCarTierLucky
		} else {
			tiers[i] = FullCarTierNormal
		}
	}
	for i := len(tiers) - 1; i > 0; i-- {
		other := int(FullCarRandom(seed, "tier", uint32(i)) % uint64(i+1))
		tiers[i], tiers[other] = tiers[other], tiers[i]
	}
	awards := make([]FullCarAward, 0, len(seats))
	for i, userID := range seats {
		pool := normal
		if tiers[i] == FullCarTierLucky {
			pool = lucky
		}
		if p, ok := weightedFullCarPrize(pool, FullCarRandom(seed, "prize", uint32(i))); ok {
			awards = append(awards, FullCarAward{SeatIndex: int32(i), UserID: userID, Prize: p, Tier: tiers[i]})
		}
	}
	return awards
}

// FullCarContainsUser 用户是否已占座（重复占座判定）
func FullCarContainsUser(seats []int64, userID int64) bool {
	for _, seated := range seats {
		if seated == userID {
			return true
		}
	}
	return false
}

// AssignFullCarSeat 座位分配：requested>=0 用指定座；否则取首个空位。
// 无空位返回 -1（越界/已占由调用方按错误码区分判定）
func AssignFullCarSeat(seats []int64, requested int32) int32 {
	if requested >= 0 {
		return requested
	}
	for i, seated := range seats {
		if seated == 0 {
			return int32(i)
		}
	}
	return -1
}
