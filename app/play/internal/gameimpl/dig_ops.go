package gameimpl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"sort"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

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
	Kind             int64  `json:"kind"`                       // 1 CASH 2 POINTS 3 COUPON 5 GOODS
	CashFen          int64  `json:"cashFen,omitempty"`          // 现金金额（分）
	Points           int64  `json:"points,omitempty"`           // 积分数
	CouponTemplateNo string `json:"couponTemplateNo,omitempty"` // 券模板单号
	CouponName       string `json:"couponName,omitempty"`       // 券名称（展示）
	SkuId            int64  `json:"skuId,omitempty"`            // catalog SKU ID（kind=5）
	ItemName         string `json:"itemName,omitempty"`         // 实物名（kind=5，展示）
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
		p := digCellPrize{Kind: c.PrizeKind, CashFen: c.CashFen, Points: c.PointsAmount, CouponTemplateNo: c.CouponTemplateNo, SkuId: c.SkuId, ItemName: c.ItemName}
		item := &play.PrizeItem{Kind: c.PrizeKind, Points: c.PointsAmount, CouponTemplateNo: c.CouponTemplateNo, SkuId: c.SkuId, ItemName: c.ItemName}
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

// digPrizeItemOf 揭晓奖品 → 响应 PrizeItem（列表/记录读回组装）
func digPrizeItemOf(p digCellPrize) *play.PrizeItem {
	item := &play.PrizeItem{Kind: p.Kind, Points: p.Points, CouponTemplateNo: p.CouponTemplateNo, CouponName: p.CouponName, SkuId: p.SkuId, ItemName: p.ItemName}
	if p.Kind == prizeCash {
		item.Amount = utils.FenToYuan(p.CashFen)
	}
	return item
}

type GetDigBoxSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantDigPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantDigPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type ListDigBoxesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type ListDigRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// buildItem 记录行 → 响应条目（prizes_json 解析 + 金额转元）
func (l *ListDigRecordsLogic) buildItem(rec *gamemodelnc.DigBoxRecords) (*play.DigRecordItem, error) {
	prizes, err := digGrantPrizesOfRecord(rec.PrizesJson)
	if err != nil {
		l.Errorf("ListDigRecords parse prizes failed, recordId: %d, err: %v", rec.Id, err)
		return nil, err
	}
	items := make([]*play.PrizeItem, 0, len(prizes))
	for _, p := range prizes {
		items = append(items, digPrizeItemOf(p))
	}
	return &play.DigRecordItem{
		RecordId:    rec.Id,
		RequestId:   rec.RequestId,
		ConfigId:    rec.ConfigId,
		CellsChosen: rec.CellsChosen,
		Prizes:      items,
		Price:       utils.FenToYuan(rec.TotalPriceFen),
		Status:      rec.Status,
		CreatedAt:   rec.CreatedAt.Unix(),
	}, nil
}

type LockDigBoxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type ReleaseDigBoxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// errDigRecordDup 事务内哨兵：同 requestId 的挖宝记录已存在（另一 gid 已结算）
var errDigRecordDup = errors.New("dig record duplicated")

type SettleDigBoxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// readBackRecord 幂等/屏障跳过路径：读已有记录如实返回
func (l *SettleDigBoxLogic) readBackRecord(requestId string, userId int64) (*play.SettleDigBoxResp, error) {
	var rec gamemodelnc.DigBoxRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "挖宝记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d", requestId, rec.Id)
	return &play.SettleDigBoxResp{RecordId: rec.Id, Status: "SETTLED"}, nil
}

type SettleDigBoxRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}
