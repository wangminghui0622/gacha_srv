package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"math/bits"
	"sort"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	"gacha_srv/pkg/utils"
)

// ══════════════ M11b 挖宝箱（DIG_BOX）公共操作 ═══════════════
// 会话按 (user_id, config_id) 唯一（uk），BIGINT 位图（cell_count≤62，cell_index 为位号）；
// 满格 COMPLETED 的会话在下一次购买时回收（位图/计数清零）。
// 发奖（SAGA 06）与 campaign 同款幂等模式：grant_no 确定性派生 + 全 uk 幂等外部调用。

// 挖宝会话状态（dig_box_sessions.status）
const (
	digSessionPlaying   int64 = 1
	digSessionCompleted int64 = 2
	digSessionCanceled  int64 = 3
)

// 挖宝记录状态（dig_box_records.status）
const (
	digRecordNormal   int64 = 1
	digRecordCanceled int64 = 2
)

// digCellPrize 挖宝格奖品（prizes_json 数组元素；kind=0 空格不落 JSON）
type digCellPrize struct {
	Kind             int64  `json:"kind"`                       // 1 CASH 2 POINTS 3 COUPON
	CashFen          int64  `json:"cashFen,omitempty"`          // 现金金额（分）
	Points           int64  `json:"points,omitempty"`           // 积分数
	CouponTemplateNo string `json:"couponTemplateNo,omitempty"` // 券模板单号
	CouponName       string `json:"couponName,omitempty"`       // 券名称（展示）
}

// digCellMask 格子列表 → 位图掩码（cell_index 为位号；调用方保证索引合法）
func digCellMask(cells []int64) int64 {
	var mask int64
	for _, c := range cells {
		mask |= 1 << c
	}
	return mask
}

// digCellsOfMask 位图 → 已置位格子列表（升序）
func digCellsOfMask(mask int64) []int64 {
	cells := make([]int64, 0, bits.OnesCount64(uint64(mask)))
	for i := 0; i < 62; i++ {
		if mask&(1<<i) != 0 {
			cells = append(cells, int64(i))
		}
	}
	return cells
}

// digJoinCells 格子列表 → 逗号分隔串（升序，记录落库用）
func digJoinCells(cells []int64) string {
	sorted := append([]int64(nil), cells...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	out := ""
	for i, c := range sorted {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("%d", c)
	}
	return out
}

// digParseCells 逗号分隔串 → 格子列表
func digParseCells(s string) []int64 {
	if s == "" {
		return nil
	}
	var out []int64
	var cur int64 = -1
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if cur >= 0 {
				out = append(out, cur)
				cur = -1
			}
			continue
		}
		if s[i] >= '0' && s[i] <= '9' {
			if cur < 0 {
				cur = 0
			}
			cur = cur*10 + int64(s[i]-'0')
		}
	}
	return out
}

// digCellsValid 校验格子列表：非空、无重复、索引 < cellCount
func digCellsValid(cells []int64, cellCount int64) bool {
	if len(cells) == 0 || cellCount <= 0 {
		return false
	}
	seen := make(map[int64]bool, len(cells))
	for _, c := range cells {
		if c < 0 || c >= cellCount || seen[c] {
			return false
		}
		seen[c] = true
	}
	return true
}

// digPrizesOfCells 格子行 → 揭晓奖品（空格 prize_kind=0 跳过；券名尽力解析）。
// 返回 JSON 串（记录落库）与响应 PrizeItem 列表（顺序与 grants 的 #idx 一致）。
func digPrizesOfCells(ctx context.Context, svcCtx *svc.ServiceContext, cells []*gamemodel.DigBoxCells) (jsonStr string, items []*play.PrizeItem) {
	prizes := make([]digCellPrize, 0, len(cells))
	items = make([]*play.PrizeItem, 0, len(cells))
	for _, c := range cells {
		if c.PrizeKind == 0 {
			continue
		}
		p := digCellPrize{Kind: c.PrizeKind, CashFen: c.CashFen, Points: c.PointsAmount, CouponTemplateNo: c.CouponTemplateNo}
		item := &play.PrizeItem{Kind: c.PrizeKind, Points: c.PointsAmount, CouponTemplateNo: c.CouponTemplateNo}
		switch c.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(c.CashFen)
		case prizeCoupon:
			p.CouponName = couponNameOf(ctx, svcCtx, c.CouponTemplateNo)
			item.CouponName = p.CouponName
		}
		prizes = append(prizes, p)
		items = append(items, item)
	}
	if len(prizes) == 0 {
		return "", items
	}
	b, err := json.Marshal(prizes)
	if err != nil {
		return "", items
	}
	return string(b), items
}

// digGrantPrizesOfRecord 记录 prizes_json → 发放队列（kind=0 空格已在落库时过滤）
func digGrantPrizesOfRecord(prizesJSON string) ([]digCellPrize, error) {
	if prizesJSON == "" {
		return nil, nil
	}
	var prizes []digCellPrize
	if err := json.Unmarshal([]byte(prizesJSON), &prizes); err != nil {
		return nil, err
	}
	return prizes, nil
}

// digPrizeItemOf 揭晓奖品 → 响应 PrizeItem（列表/记录读回组装）
func digPrizeItemOf(p digCellPrize) *play.PrizeItem {
	item := &play.PrizeItem{Kind: p.Kind, Points: p.Points, CouponTemplateNo: p.CouponTemplateNo, CouponName: p.CouponName}
	if p.Kind == prizeCash {
		item.Amount = utils.FenToYuan(p.CashFen)
	}
	return item
}
