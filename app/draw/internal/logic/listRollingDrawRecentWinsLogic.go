package logic

import (
	"context"
	"time"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/core"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRollingDrawRecentWinsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRollingDrawRecentWinsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRollingDrawRecentWinsLogic {
	return &ListRollingDrawRecentWinsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 滚动中奖 feed 常量（server_v2 rolling_draw 同款）
const (
	rollingFeedDefaultLimit = 20  // 全量默认条数
	rollingFeedMaxLimit     = 50  // 上限
	rollingMarqueeDefault   = 10  // marqueeOnly 默认条数
	rollingScanFactor       = 10  // marquee 过滤 NORMAL 后扫描倍数
	rollingScanCap          = 500 // 扫描行数上限
)

// rollingWinRow 记录 join 条目（展示价/限量）+ 盒（规则）的 feed 行
type rollingWinRow struct {
	RequestId     string    `gorm:"column:request_id"`
	UserId        int64     `gorm:"column:user_id"`
	BoxId         int64     `gorm:"column:box_id"`
	EntryName     string    `gorm:"column:entry_name"`
	DisplayValFen int64     `gorm:"column:display_value_fen"`
	RollingLimit  int64     `gorm:"column:rolling_limited"`
	RulesJSON     *string   `gorm:"column:rolling_rules_json"`
	WonAt         time.Time `gorm:"column:won_at"`
}

// ListRollingDrawRecentWins 滚动中奖最近中奖列表（首页 marquee 数据源）。
// draw_records 即 feed（id DESC join entries 取展示价/限量 + 盒规则取阈值，逐行算等级）；
// marqueeOnly 只回 HIGH_VALUE/LIMITED，扫描 10x（上限 500 行）兜底过滤 NORMAL。
// 普通 RPC（非 SAGA 分支）：业务错误返回 CodeError。
func (l *ListRollingDrawRecentWinsLogic) ListRollingDrawRecentWins(in *draw.ListRollingDrawRecentWinsReq) (*draw.ListRollingDrawRecentWinsResp, error) {
	defaultLimit := int64(rollingFeedDefaultLimit)
	if in.MarqueeOnly {
		defaultLimit = rollingMarqueeDefault
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > rollingFeedMaxLimit {
		limit = rollingFeedMaxLimit
	}
	scan := limit * rollingScanFactor
	if scan > rollingScanCap {
		scan = rollingScanCap
	}

	// 记录 id DESC 扫描（join 条目与盒规则）；uk(box_id,name) 保证条目 join 1:1
	var rows []rollingWinRow
	q := l.svcCtx.DB.WithContext(l.ctx).
		Table("draw_records AS r").
		Select("r.request_id, r.user_id, r.box_id, r.entry_name, r.created_at AS won_at, "+
			"e.display_value_fen, e.rolling_limited, b.rolling_rules_json").
		Joins("LEFT JOIN draw_pool_entries AS e ON e.box_id = r.box_id AND e.name = r.entry_name").
		Joins("LEFT JOIN draw_boxes AS b ON b.id = r.box_id").
		Where("r.status = 1").
		Order("r.id DESC").
		Limit(int(scan))
	if in.BoxId > 0 {
		q = q.Where("r.box_id = ?", in.BoxId)
	}
	if err := q.Find(&rows).Error; err != nil {
		l.Errorf("ListRollingDrawRecentWins query failed, err: %v", err)
		return nil, errorx.FromMySQLError(err)
	}

	// 逐行算等级：盒阈值按 boxId 缓存解析；marquee 过滤 NORMAL
	wins := make([]*draw.RollingDrawWinItem, 0, limit)
	thresholdCache := make(map[int64]int64, 4)
	for i := range rows {
		if int64(len(wins)) >= limit {
			break
		}
		r := &rows[i]
		th, ok := thresholdCache[r.BoxId]
		if !ok {
			th = 0
			if r.RulesJSON != nil && *r.RulesJSON != "" {
				rules, err := core.ParseRollingRules(*r.RulesJSON)
				if err != nil {
					l.Errorf("rolling rules invalid, boxId: %d, err: %v", r.BoxId, err)
				} else {
					th = rules.HighValueThresholdFen
				}
			}
			thresholdCache[r.BoxId] = th
		}
		tier := core.RollingTier(core.RollingEntry{Id: r.BoxId, Limited: r.RollingLimit, DisplayVal: r.DisplayValFen}, th)
		if in.MarqueeOnly && tier == core.TierNormal {
			continue // marquee 只滚动高档/限量（server_v2 同款过滤）
		}
		wins = append(wins, &draw.RollingDrawWinItem{
			RequestId:       r.RequestId,
			UserId:          r.UserId,
			BoxId:           r.BoxId,
			EntryName:       r.EntryName,
			DisplayValueFen: r.DisplayValFen,
			Tier:            tier,
			WonAt:           r.WonAt.Unix(),
		})
	}

	// 指定 boxId 时回该盒参考阈值（前端高档彩标判定用；无胜场时也能取到）
	var threshold int64
	if in.BoxId > 0 {
		if b, err := l.svcCtx.BoxModel.FindOne(l.ctx, in.BoxId); err == nil &&
			b.RollingRulesJson != nil && *b.RollingRulesJson != "" {
			if rules, e := core.ParseRollingRules(*b.RollingRulesJson); e == nil {
				threshold = rules.HighValueThresholdFen
			}
		}
	}
	return &draw.ListRollingDrawRecentWinsResp{
		Wins:                  wins,
		HighValueThresholdFen: threshold,
	}, nil
}
