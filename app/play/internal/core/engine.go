// Package core 游戏玩法确定性核心：加权抽取（regular 奖）与整车/扭蛋结算纯函数。
// 随机源与模型层解耦：regular 奖用 SHA256(seed‖domain‖BE32(index)) 派生（FullCarRandom），
// 整车/扭蛋结算见 fullcar.go / gashapon.go（同为确定性）。
// 本包不再使用 math/rand：全部随机都从种子确定性派生，保证公平性可复算。
package core

// PrizeEntry 参与抽取的条目（与模型层解耦，仅引擎需要的字段）
type PrizeEntry struct {
	Id     int64
	Name   string
	Weight int
}

// PickPrizeDeterministic 确定性加权抽取：random = FullCarRandom(seed, domain, index)，
// selected = random % total，累计扣减（与 fullcar.go weightedFullCarPrize 同款算法）。
// 入参必须是「可抽取」条目（调用方负责排除售罄项）；空池返回 nil。
// seed 由调用方注入（满车/扭蛋 regular 奖用全局事务 gid），同一 (entries, seed, domain, index)
// 必然抽到同一条目，可离线复算验证。
func PickPrizeDeterministic(entries []PrizeEntry, seed []byte, domain string, index uint32) *PrizeEntry {
	if len(entries) == 0 {
		return nil
	}
	var total uint64
	for _, e := range entries {
		total += uint64(e.Weight)
	}
	if total == 0 {
		return nil
	}
	selected := FullCarRandom(seed, domain, index) % total
	for i := range entries {
		w := uint64(entries[i].Weight)
		if selected < w {
			return &entries[i]
		}
		selected -= w
	}
	return &entries[len(entries)-1]
}
