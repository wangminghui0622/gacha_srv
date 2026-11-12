package gameimpl

import (
	"context"
	"encoding/json"
	"errors"
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type GetGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantGroupBuyPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantGroupBuyPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// 拼团抽状态（group_buy_state.status）
const (
	groupBuyStateOpen    int64 = 1
	groupBuyStateSettled int64 = 2
)

// 拼团抽记录状态（group_buy_records.status）
const (
	groupBuyRecordNormal   int64 = 1
	groupBuyRecordCanceled int64 = 2
)

// groupBuyStateJSON state 行三段 JSON 内存视图
type groupBuyStateJSON struct {
	Bets      []core.GroupBuyBet   `json:"bets"`
	Awards    []core.GroupBuyAward `json:"awards"`
	LuckySent []int32              `json:"lucky_sent"`
}

// groupBuyParseState state 行三段 JSON → 内存状态（空串取零值；坏 JSON → 错误）
func groupBuyParseState(betsJson, awardsJson, luckySentJson string) (*groupBuyStateJSON, error) {
	state := &groupBuyStateJSON{}
	if betsJson != "" {
		if err := json.Unmarshal([]byte(betsJson), &state.Bets); err != nil {
			return nil, err
		}
	}
	if state.Bets == nil {
		state.Bets = []core.GroupBuyBet{}
	}
	if awardsJson != "" {
		if err := json.Unmarshal([]byte(awardsJson), &state.Awards); err != nil {
			return nil, err
		}
	}
	if state.Awards == nil {
		state.Awards = []core.GroupBuyAward{}
	}
	if luckySentJson != "" {
		if err := json.Unmarshal([]byte(luckySentJson), &state.LuckySent); err != nil {
			return nil, err
		}
	}
	if state.LuckySent == nil {
		state.LuckySent = []int32{}
	}
	return state, nil
}

// groupBuyParseLucky lucky_prizes_json → 幸运奖配置（Remaining=quantity 原始值；
// 结算输入侧由调用方按 lucky_sent[prize_index] 扣减；空/坏 JSON → 错误，调用方报 3020）
func groupBuyParseLucky(jsonStr string) ([]core.GroupBuyLuckyStock, error) {
	if jsonStr == "" {
		return nil, errors.New("empty lucky prizes json")
	}
	var stocks []core.GroupBuyLuckyStock
	if err := json.Unmarshal([]byte(jsonStr), &stocks); err != nil {
		return nil, err
	}
	return stocks, nil
}

// groupBuyParseLuckyChildren lucky_children_json → 幸运子奖配置（空 → nil；坏 JSON → 错误）
func groupBuyParseLuckyChildren(jsonStr string) ([]core.GroupBuyLuckyChildStock, error) {
	if jsonStr == "" {
		return nil, nil
	}
	var children []core.GroupBuyLuckyChildStock
	if err := json.Unmarshal([]byte(jsonStr), &children); err != nil {
		return nil, err
	}
	return children, nil
}

// groupBuyBetsJSON 下注列表 → JSON（[]struct 恒可编码）
func groupBuyBetsJSON(bets []core.GroupBuyBet) string {
	b, _ := json.Marshal(bets)
	return string(b)
}

// groupBuyAwardsJSON 结算奖列表 → JSON（[]struct 恒可编码）
func groupBuyAwardsJSON(awards []core.GroupBuyAward) string {
	b, _ := json.Marshal(awards)
	return string(b)
}

// groupBuyLuckySentJSON 幸运已发计数 → JSON（[]int32 恒可编码）
func groupBuyLuckySentJSON(sent []int32) string {
	b, _ := json.Marshal(sent)
	return string(b)
}

// groupBuyAwardsOfRequest 取指定 requestId 归属的结算奖（06 发放本 gid 触发的结算奖）
func groupBuyAwardsOfRequest(awards []core.GroupBuyAward, requestId string) []core.GroupBuyAward {
	out := make([]core.GroupBuyAward, 0, len(awards))
	for _, a := range awards {
		if a.RequestID == requestId {
			out = append(out, a)
		}
	}
	return out
}

type ListGroupBuyRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// buildItem 记录行 → 响应条目（金额转元；结算奖按 requestId 过滤自 state.awards_json）
func (l *ListGroupBuyRecordsLogic) buildItem(rec *gamemodelnc.GroupBuyRecords) *play.GroupBuyRecordItem {
	item := &play.GroupBuyRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		Count:     rec.Count,
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		if state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson); err == nil {
			for _, a := range groupBuyAwardsOfRequest(state.Awards, rec.RequestId) {
				item.Awards = append(item.Awards, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
			}
		}
	}
	return item
}

type LockGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// countOfRequest 幂等/屏障跳过路径：按 requestId 读下注记录如实返回；未找到 → 回退请求值
func (l *LockGroupBuyLogic) countOfRequest(db *gorm.DB, configId int64, requestId string, fallback int64) int64 {
	var st gamemodelnc.GroupBuyState
	if err := db.Where("config_id = ?", configId).First(&st).Error; err != nil {
		return fallback
	}
	state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
	if err != nil {
		return fallback
	}
	for _, b := range state.Bets {
		if b.DrawID == requestId {
			return int64(b.Count)
		}
	}
	return fallback
}

type ReleaseGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// errGroupBuyRecordDup 事务内哨兵：同 requestId 的拼团记录已存在（另一 gid 已结算）
var errGroupBuyRecordDup = errors.New("group buy record duplicated")

type SettleGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// groupBuyAwardItems 结算奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleGroupBuyLogic) groupBuyAwardItems(awards []core.GroupBuyAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		items = append(items, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 结算奖 + 当前状态如实返回
func (l *SettleGroupBuyLogic) readBackRecord(requestId string, userId int64) (*play.SettleGroupBuyResp, error) {
	var rec gamemodelnc.GroupBuyRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "拼团记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var awards []core.GroupBuyAward
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == groupBuyStateSettled
		if state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson); err == nil {
			awards = groupBuyAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, awards: %d", requestId, rec.Id, settled, len(awards))
	return &play.SettleGroupBuyResp{
		RecordId: rec.Id,
		Awards:   l.groupBuyAwardItems(awards),
		Settled:  settled,
		Status:   "SETTLED",
	}, nil
}

type SettleGroupBuyRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}
