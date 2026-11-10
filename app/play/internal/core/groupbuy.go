package core

import (
	"errors"
	"fmt"
	"sort"
)

// M13 拼团抽（group_buy）结算纯函数：语义逐条移植 server_v2 biz/group_buy_draw_settlement.go。
// 随机源 = GroupBuyRandom = FullCarRandom（SHA256 派生）；本仓 seed = 全局事务 gid。
//
// 结算流程（最后一注触发，SettleGroupBuyBatch 同款）：
//   ① 普通槽 = 普通条目（parent_entry_id=0）按 remaining 展开，Fisher-Yates 洗牌（"group-buy-slot"）；
//   ② perBet[target]=1 打底；slotCount>target 时超量槽逐槽随机派给 count<limit 的下注
//      （"group-buy-extra"；limit=per_bet_award_limit，0=默认 slotCount-target+1）；
//   ③ 按 bets 顺序、每注 perBet[全局注号] 槽数，游标依次取洗牌后的槽（槽必须全部分完）；
//      父条目抽中 → 条目子奖（parent_entry_id 关联）发 1 件（remaining 扣减，随函数内推进）；
//   ④ 幸运奖：池=用户按下注数展开、按 userId 升序、洗牌（"group-buy-lucky-pool"），
//      游标 winner=pool[cursor%len] 逐件发放；幸运子奖随幸运奖逐件发放。
// 偏离 server_v2：跳过 ApplyGroupBuyNetBets 钱包流水重建（SAGA Release 直接弹 bet 记录）。
// 库存推进由调用方按结算奖反推（normal/child 按 entry_id 计数、lucky 按 lucky_index 计数）。

// 结算奖类型（awards_json 的 kind）
const (
	GroupBuyAwardKindNormal = "normal"
	GroupBuyAwardKindChild  = "child"
	GroupBuyAwardKindLucky  = "lucky"
)

// GroupBuyBet 下注记录（bets_json 元素；DrawID=requestId）
type GroupBuyBet struct {
	UserID int64  `json:"user_id"`
	DrawID string `json:"draw_id"`
	Count  int32  `json:"count"`
}

// GroupBuyAward 结算奖（state.awards_json 元素；EntryID/LuckyIndex 供补偿回补库存）
type GroupBuyAward struct {
	UserID     int64  `json:"user_id"`
	Kind       string `json:"kind"` // normal / child / lucky
	EntryID    int64  `json:"entry_id,omitempty"`
	EntryName  string `json:"entry_name,omitempty"`  // normal：展示名（谢谢参与无资产）
	LuckyIndex int32  `json:"lucky_index,omitempty"` // lucky：幸运奖下标
	RequestID  string `json:"request_id,omitempty"`
	PrizeShape
}

// GroupBuySlot 普通槽（结算输入；按条目 remaining 展开，每槽=1 份）
type GroupBuySlot struct {
	EntryID   int64
	EntryName string
	IsPrize   bool
	Remaining int32 // 展开时的剩余库存（本次结算调用方先行扣减依据）
}

// GroupBuyChildStock 条目子奖库存（parent_entry_id 关联父条目）
type GroupBuyChildStock struct {
	EntryID   int64
	ParentID  int64
	Remaining int32
	PrizeShape
}

// GroupBuyLuckyChildStock 幸运子奖配置（lucky_children_json 按下标对齐；
// 无持久库存——每件幸运奖随发一次，配置 quantity 仅展示，与连击链连击奖同款处理）
type GroupBuyLuckyChildStock struct {
	Index int32 `json:"lucky_index"`
	PrizeShape
}

// GroupBuyLuckyStock 幸运奖库存（lucky_prizes_json 按下标对齐；
// JSON 键名对齐配置 DDL：prize_index/quantity；Remaining 在结算输入侧 =
// 配置 quantity - lucky_sent[prize_index]，由调用方扣减）
type GroupBuyLuckyStock struct {
	Index     int32 `json:"prize_index"`
	Remaining int32 `json:"quantity"`
	Children  []GroupBuyLuckyChildStock
	PrizeShape
}

// GroupBuyRandom 拼团抽确定性随机数（复用满车同款派生）
func GroupBuyRandom(seed []byte, domain string, index uint32) uint64 {
	return FullCarRandom(seed, domain, index)
}

// SettleGroupBuyBatch 整批结算（server_v2 SettleGroupBuyBatch 同款）。
// bets 按下注顺序（总注数必须==target）；slots 为普通槽展开（余量须≥target）；
// children/lucky 为子奖/幸运奖库存快照（函数内推进 remaining，调用方按结算奖计数落库）。
// 返回结算奖（不含 RequestID，由调用方标注触发 gid）。
func SettleGroupBuyBatch(bets []GroupBuyBet, target int, perBetLimit int, slots []GroupBuySlot,
	children []GroupBuyChildStock, lucky []GroupBuyLuckyStock, seed []byte) ([]GroupBuyAward, error) {
	var total int32
	for _, bet := range bets {
		total += bet.Count
	}
	if int(total) != target {
		return nil, fmt.Errorf("group buy batch bet total mismatch: got %d want %d", total, target)
	}
	if len(slots) < target {
		return nil, errors.New("group buy normal slots are insufficient for target bets")
	}

	// 库存快照深拷贝：结算函数不修改入参（调用方按结算奖计数自行落库）
	childrenCopy := append([]GroupBuyChildStock(nil), children...)
	luckyCopy := append([]GroupBuyLuckyStock(nil), lucky...)
	for i := range luckyCopy {
		luckyCopy[i].Children = append([]GroupBuyLuckyChildStock(nil), lucky[i].Children...)
	}

	// ① 洗牌普通槽（Fisher-Yates，server_v2 同款 i 从 len-1 到 1）
	shuffled := append([]GroupBuySlot(nil), slots...)
	if len(shuffled) > 1 {
		for i := len(shuffled) - 1; i > 0; i-- {
			j := int(GroupBuyRandom(seed, "group-buy-slot", uint32(i)) % uint64(i+1))
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		}
	}

	// ② perBet 分配：每注 1 槽打底，超量槽随机派给 count<limit 的下注
	perBet := make([]int32, target)
	for i := range perBet {
		perBet[i] = 1
	}
	if slotCount := len(slots); slotCount > target {
		limit := perBetLimit
		if limit == 0 {
			limit = slotCount - target + 1
		}
		if limit < 1 {
			limit = 1
		}
		extra := slotCount - target
		for i := 0; i < extra; i++ {
			eligible := make([]int, 0, len(perBet))
			for idx, count := range perBet {
				if int(count) < limit {
					eligible = append(eligible, idx)
				}
			}
			if len(eligible) == 0 {
				return nil, fmt.Errorf("group buy per-bet award limit exhausted at %d", limit)
			}
			pick := eligible[int(GroupBuyRandom(seed, "group-buy-extra", uint32(i))%uint64(len(eligible)))]
			perBet[pick]++
		}
	}

	// ③ 按注分配槽 + 条目子奖
	awards := make([]GroupBuyAward, 0, len(slots)+len(lucky))
	cursor := 0
	for betIndex, bet := range bets {
		var globalBetNo int32
		for _, previous := range bets[:betIndex] {
			globalBetNo += previous.Count
		}
		for i := int32(0); i < bet.Count; i++ {
			for j := int32(0); j < perBet[globalBetNo]; j++ {
				if cursor >= len(shuffled) {
					return nil, errors.New("group buy slot cursor overflow")
				}
				slot := shuffled[cursor]
				cursor++
				awards = append(awards, GroupBuyAward{
					UserID:    bet.UserID,
					Kind:      GroupBuyAwardKindNormal,
					EntryID:   slot.EntryID,
					EntryName: slot.EntryName,
				})
				// 条目子奖：父条目抽中发 1 件（remaining 扣减）
				for ci := range childrenCopy {
					child := &childrenCopy[ci]
					if child.ParentID != slot.EntryID || child.Remaining <= 0 {
						continue
					}
					awards = append(awards, GroupBuyAward{
						UserID:     bet.UserID,
						Kind:       GroupBuyAwardKindChild,
						EntryID:    child.EntryID,
						PrizeShape: child.PrizeShape,
					})
					child.Remaining--
				}
			}
			globalBetNo++
		}
	}
	if cursor != len(shuffled) {
		return nil, errors.New("group buy slots were not fully assigned")
	}

	// ④ 幸运奖：用户按下注数展开池、洗牌、游标逐件
	luckyAwards, err := settleGroupBuyLucky(bets, luckyCopy, seed)
	if err != nil {
		return nil, err
	}
	awards = append(awards, luckyAwards...)
	return awards, nil
}

// settleGroupBuyLucky 幸运奖加权池（server_v2 settleGroupBuyLucky 同款）
func settleGroupBuyLucky(bets []GroupBuyBet, lucky []GroupBuyLuckyStock, seed []byte) ([]GroupBuyAward, error) {
	type weighted struct {
		userID int64
		weight int32
	}
	userBets := map[int64]int32{}
	for _, bet := range bets {
		userBets[bet.UserID] += bet.Count
	}
	weights := make([]weighted, 0, len(userBets))
	for userID, count := range userBets {
		weights = append(weights, weighted{userID: userID, weight: count})
	}
	sort.Slice(weights, func(i, j int) bool { return weights[i].userID < weights[j].userID })
	pool := make([]int64, 0)
	for _, item := range weights {
		for i := int32(0); i < item.weight; i++ {
			pool = append(pool, item.userID)
		}
	}
	if len(pool) > 1 {
		for i := len(pool) - 1; i > 0; i-- {
			j := int(GroupBuyRandom(seed, "group-buy-lucky-pool", uint32(i)) % uint64(i+1))
			pool[i], pool[j] = pool[j], pool[i]
		}
	}
	cursor := 0
	awards := make([]GroupBuyAward, 0)
	for li := range lucky {
		stock := &lucky[li]
		for stock.Remaining > 0 {
			if len(pool) == 0 {
				return nil, errors.New("group buy lucky pool is empty")
			}
			winner := pool[cursor%len(pool)]
			cursor++
			awards = append(awards, GroupBuyAward{
				UserID:     winner,
				Kind:       GroupBuyAwardKindLucky,
				LuckyIndex: stock.Index,
				PrizeShape: stock.PrizeShape,
			})
			stock.Remaining--
			// 幸运子奖：随每件幸运奖发放（无库存，配置数量仅展示）
			for ci := range stock.Children {
				child := &stock.Children[ci]
				awards = append(awards, GroupBuyAward{
					UserID:     winner,
					Kind:       GroupBuyAwardKindChild,
					LuckyIndex: stock.Index,
					PrizeShape: child.PrizeShape,
				})
			}
		}
	}
	return awards, nil
}
