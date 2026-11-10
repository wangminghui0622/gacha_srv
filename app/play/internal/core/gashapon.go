package core

import (
	"sort"
)

// M12c 扭蛋（gashapon）特别奖判定：语义逐条移植自 server_v2 biz/gashapon.go。
// 随机源 = SHA256(seed‖domain‖BE32(index)) 前 8 字节大端（复用满车 FullCarRandom）；
// 本仓 seed = 全局事务 gid（requestId），同一 (state, prizes, seed) 结果确定可回放。
//
// 票据模型：records 按序构成票据区间（1 起）——record i 覆盖 [start, start+count)；
// 抽数 drawn = total - remaining（state 余票为 03 锁定后的值）。
// 分阶段 LUCKY：单位按 (stage_order, prize_index, unit) 排序，位置 p 阈值 T=(p+1)*total/m，
// drawn ≥ T 且 unit 未发 → 单元 0 且指派者触及 [prev+1..T] 区间 → 指派者中；
// 否则 winningTicket = prev+1 + rand % (T-prev) 的票据主人。
// 清空抽（remaining==0）追加终局：LAST 全给清空者；MVP 按参与数取（并列取最小 userId，
// 超出单元数截断）；LUCKY 剩余先看指派者（须参与过）取首单元，其余按参与数加权随机。

// 特别奖类型（special_prizes_json 的 special_kind）
const (
	GashaponKindLast  = "LAST"
	GashaponKindMVP   = "MVP"
	GashaponKindLucky = "LUCKY"
)

// GashaponSpecialPrize 特别奖配置（gashapon_configs.special_prizes_json 数组元素）
type GashaponSpecialPrize struct {
	PrizeIndex       int32  `json:"prize_index"`
	SpecialKind      string `json:"special_kind"` // LAST / MVP / LUCKY
	PrizeKind        int64  `json:"prize_kind"`   // 1 CASH 2 POINTS 3 COUPON
	CashFen          int64  `json:"cash_fen"`
	PointsAmount     int64  `json:"points_amount"`
	CouponTemplateNo string `json:"coupon_template_no"`
	Quantity         int32  `json:"quantity"`
	StageOrder       int32  `json:"stage_order"` // 0=不分阶段（LUCKY 专用）
}

// GashaponParticipation 参与记录（records 按序=票据区间，1 起）
type GashaponParticipation struct {
	UserID    int64  `json:"user_id"`
	RequestID string `json:"request_id"`
	Count     int64  `json:"count"`
}

// GashaponAward 特别奖结果（awards_json 元素；RequestID 供补偿精确定位）
type GashaponAward struct {
	PrizeIndex       int32  `json:"prize_index"`
	Unit             int32  `json:"unit"`
	UserID           int64  `json:"user_id"`
	SpecialKind      string `json:"special_kind"`
	PrizeKind        int64  `json:"prize_kind"` // 1 CASH 2 POINTS 3 COUPON
	CashFen          int64  `json:"cash_fen"`
	PointsAmount     int64  `json:"points_amount"`
	CouponTemplateNo string `json:"coupon_template_no"`
	StageOrder       int32  `json:"stage_order,omitempty"`
	RequestID        string `json:"request_id"`
}

// GashaponBoxState 扭蛋状态（state 表 participants/records/sent/awards JSON 结构）
type GashaponBoxState struct {
	Participants map[int64]int64         `json:"participants"`
	Records      []GashaponParticipation `json:"records"`
	Sent         []int32                 `json:"sent"`
	Awards       []GashaponAward         `json:"awards,omitempty"`
}

// gashaponRandom 确定性随机数（复用满车同款派生：SHA256(seed‖domain‖BE32(index)) 前 8 字节大端）
func gashaponRandom(seed []byte, domain string, index uint32) uint64 {
	return FullCarRandom(seed, domain, index)
}

type stagedPrizeUnit struct {
	prizeIndex int
	stageOrder int32
	unit       int32
}

// StagedGashaponAwards 分阶段 LUCKY 判定（server_v2 stagedGashaponAwards 同款）。
// total=总票数、drawn=已抽数（含本次）；state.Sent 在函数内推进（调用方落库）。
// assignees：prize_index → 指派用户（仅 LUCKY 单元 0 且触及当前阶段区间才生效）。
func StagedGashaponAwards(state *GashaponBoxState, prizes []GashaponSpecialPrize, total, drawn int64, seed []byte, assignees map[int32]int64) []GashaponAward {
	units := make([]stagedPrizeUnit, 0)
	for index, prize := range prizes {
		if prize.SpecialKind == GashaponKindLucky && prize.StageOrder > 0 {
			for unit := int32(0); unit < prize.Quantity; unit++ {
				units = append(units, stagedPrizeUnit{prizeIndex: index, stageOrder: prize.StageOrder, unit: unit})
			}
		}
	}
	sort.Slice(units, func(i, j int) bool {
		if units[i].stageOrder != units[j].stageOrder {
			return units[i].stageOrder < units[j].stageOrder
		}
		if units[i].prizeIndex != units[j].prizeIndex {
			return units[i].prizeIndex < units[j].prizeIndex
		}
		return units[i].unit < units[j].unit
	})
	if len(units) == 0 {
		return nil
	}
	awards := make([]GashaponAward, 0)
	for position, item := range units {
		if state.Sent[item.prizeIndex] > item.unit {
			continue
		}
		threshold := int64((int64(position+1) * total) / int64(len(units)))
		if drawn < threshold {
			continue
		}
		previous := int64((int64(position) * total) / int64(len(units)))
		minTicket := previous + 1
		if threshold < minTicket {
			continue
		}
		var winner int64
		if item.unit == 0 {
			if user := assignees[int32(item.prizeIndex)]; user != 0 && gashaponUserTouchesRange(state.Records, user, minTicket, threshold) {
				winner = user
			}
		}
		if winner == 0 {
			winningTicket := minTicket + int64(gashaponRandom(seed, "stage", uint32(position))%uint64(threshold-minTicket+1))
			winner = gashaponTicketOwner(state.Records, winningTicket)
		}
		if winner == 0 {
			continue
		}
		prize := prizes[item.prizeIndex]
		awards = append(awards, GashaponAward{
			PrizeIndex:       int32(item.prizeIndex),
			Unit:             item.unit,
			UserID:           winner,
			SpecialKind:      prize.SpecialKind,
			PrizeKind:        prize.PrizeKind,
			CashFen:          prize.CashFen,
			PointsAmount:     prize.PointsAmount,
			CouponTemplateNo: prize.CouponTemplateNo,
			StageOrder:       prize.StageOrder,
		})
		state.Sent[item.prizeIndex]++
	}
	return awards
}

// FinalGashaponAwards 清空抽终局判定（server_v2 finalGashaponAwards 同款）。
// lastUser=清空者；state.Sent 在函数内推进；LUCKY 剩余先看指派者（须参与过）取首单元。
func FinalGashaponAwards(state *GashaponBoxState, prizes []GashaponSpecialPrize, lastUser int64, seed []byte, assignees map[int32]int64) []GashaponAward {
	awards := make([]GashaponAward, 0)
	maxUsers := gashaponMVPUsers(state.Participants)
	for index, prize := range prizes {
		firstRemaining := true
		for state.Sent[index] < prize.Quantity {
			unit := state.Sent[index]
			var winner int64
			switch prize.SpecialKind {
			case GashaponKindLast:
				winner = lastUser
			case GashaponKindMVP:
				if int(unit) < len(maxUsers) {
					winner = maxUsers[unit]
				}
			case GashaponKindLucky:
				if firstRemaining {
					if user := assignees[int32(index)]; user != 0 && state.Participants[user] > 0 {
						winner = user
					}
				}
				if winner == 0 {
					winner = weightedGashaponUser(state.Participants, gashaponRandom(seed, "lucky", uint32(index)<<16|uint32(unit)))
				}
			}
			firstRemaining = false
			if winner == 0 {
				break
			}
			awards = append(awards, GashaponAward{
				PrizeIndex:       int32(index),
				Unit:             unit,
				UserID:           winner,
				SpecialKind:      prize.SpecialKind,
				PrizeKind:        prize.PrizeKind,
				CashFen:          prize.CashFen,
				PointsAmount:     prize.PointsAmount,
				CouponTemplateNo: prize.CouponTemplateNo,
				StageOrder:       prize.StageOrder,
			})
			state.Sent[index]++
		}
	}
	return awards
}

// gashaponMVPUsers 最大参与数用户列表（并列取最小 userId 优先；server_v2 同款）
func gashaponMVPUsers(participants map[int64]int64) []int64 {
	var max int64
	users := make([]int64, 0)
	for userID, count := range participants {
		if count > max {
			max, users = count, []int64{userID}
		} else if count == max && max > 0 {
			users = append(users, userID)
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i] < users[j] })
	return users
}

// weightedGashaponUser 按参与数加权随机取用户（用户列表按 userId 升序，与 map 遍历序无关）
func weightedGashaponUser(participants map[int64]int64, random uint64) int64 {
	users := make([]int64, 0, len(participants))
	var total uint64
	for userID, count := range participants {
		users, total = append(users, userID), total+uint64(count)
	}
	sort.Slice(users, func(i, j int) bool { return users[i] < users[j] })
	if total == 0 {
		return 0
	}
	selected := random % total
	for _, userID := range users {
		weight := uint64(participants[userID])
		if selected < weight {
			return userID
		}
		selected -= weight
	}
	return 0
}

// gashaponUserTouchesRange 用户是否触及票据区间 [minTicket, maxTicket]（闭区间；票据 1 起）
func gashaponUserTouchesRange(records []GashaponParticipation, user, minTicket, maxTicket int64) bool {
	var start int64
	for _, record := range records {
		end := start + record.Count
		if record.UserID == user && end >= minTicket && start < maxTicket {
			return true
		}
		start = end
	}
	return false
}

// gashaponTicketOwner 票据主人（records 按序；票据 1 起，越界返回 0）
func gashaponTicketOwner(records []GashaponParticipation, ticket int64) int64 {
	var end int64
	for _, record := range records {
		end += record.Count
		if ticket <= end {
			return record.UserID
		}
	}
	return 0
}
