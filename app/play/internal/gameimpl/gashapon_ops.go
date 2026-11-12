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

type AssignGashaponLuckyUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// 扭蛋状态（gashapon_state.status）
const (
	gashaponStateOpen    int64 = 1
	gashaponStateSettled int64 = 2
)

// 扭蛋记录状态（gashapon_records.status）
const (
	gashaponRecordNormal   int64 = 1
	gashaponRecordCanceled int64 = 2
)

// gashaponOutcome regular 奖结果落库元素（entries_json 数组元素；谢谢参与无资产，仅名字+是否中奖）。
// EntryId：条目主键——05 补偿回库存按主键精确定位（name 可重复/被改名，
// config_id+name 匹配会错还库存；旧记录无 entry_id 时回退 name 匹配）
type gashaponOutcome struct {
	EntryId int64  `json:"entry_id"`
	Name    string `json:"name"`
	IsPrize bool   `json:"is_prize"`
}

// gashaponParseState state 行四段 JSON → 内存状态（空串取零值；坏 JSON → 错误）
func gashaponParseState(participantsJson, recordsJson, sentJson, awardsJson string) (*core.GashaponBoxState, error) {
	state := &core.GashaponBoxState{}
	if participantsJson != "" {
		if err := json.Unmarshal([]byte(participantsJson), &state.Participants); err != nil {
			return nil, err
		}
	}
	if state.Participants == nil {
		state.Participants = map[int64]int64{}
	}
	if recordsJson != "" {
		if err := json.Unmarshal([]byte(recordsJson), &state.Records); err != nil {
			return nil, err
		}
	}
	if sentJson != "" {
		if err := json.Unmarshal([]byte(sentJson), &state.Sent); err != nil {
			return nil, err
		}
	}
	if awardsJson != "" {
		if err := json.Unmarshal([]byte(awardsJson), &state.Awards); err != nil {
			return nil, err
		}
	}
	return state, nil
}

// gashaponParsePrizes special_prizes_json → 特别奖配置（空/坏 JSON → 错误，调用方报 3020）
func gashaponParsePrizes(jsonStr string) ([]core.GashaponSpecialPrize, error) {
	if jsonStr == "" {
		return nil, errors.New("empty special prizes json")
	}
	var prizes []core.GashaponSpecialPrize
	if err := json.Unmarshal([]byte(jsonStr), &prizes); err != nil {
		return nil, err
	}
	return prizes, nil
}

// gashaponParseOutcomes entries_json → regular 奖结果（空串/坏 JSON → 空列表，防御配置垃圾）
func gashaponParseOutcomes(jsonStr string) []gashaponOutcome {
	if jsonStr == "" {
		return nil
	}
	var outcomes []gashaponOutcome
	if err := json.Unmarshal([]byte(jsonStr), &outcomes); err != nil {
		return nil
	}
	return outcomes
}

// gashaponOutcomesJSON regular 奖结果 → JSON（[]struct 恒可编码）
func gashaponOutcomesJSON(outcomes []gashaponOutcome) string {
	b, _ := json.Marshal(outcomes)
	return string(b)
}

// gashaponAwardsJSON 特别奖列表 → JSON（[]struct 恒可编码）
func gashaponAwardsJSON(awards []core.GashaponAward) string {
	b, _ := json.Marshal(awards)
	return string(b)
}

// gashaponAwardsOfRequest 取指定 requestId 归属的特别奖（06 发放本 gid 触发的特别奖）
func gashaponAwardsOfRequest(awards []core.GashaponAward, requestId string) []core.GashaponAward {
	out := make([]core.GashaponAward, 0, len(awards))
	for _, a := range awards {
		if a.RequestID == requestId {
			out = append(out, a)
		}
	}
	return out
}

// gashaponSentJSON sent 计数 → JSON（[]int32 恒可编码）
func gashaponSentJSON(sent []int32) string {
	b, _ := json.Marshal(sent)
	return string(b)
}

// gashaponParticipantsJSON 参与数 map → JSON（map[int64]int64 恒可编码）
func gashaponParticipantsJSON(participants map[int64]int64) string {
	b, _ := json.Marshal(participants)
	return string(b)
}

// gashaponRecordsJSON 参与记录 → JSON（[]struct 恒可编码）
func gashaponRecordsJSON(records []core.GashaponParticipation) string {
	b, _ := json.Marshal(records)
	return string(b)
}

type GetGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantGashaponPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantGashaponPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type ListGashaponRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// buildItem 记录行 → 响应条目（金额转元；entries_json → 结果条目）
func (l *ListGashaponRecordsLogic) buildItem(rec *gamemodelnc.GashaponRecords) *play.GashaponRecordItem {
	return &play.GashaponRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		Count:     rec.Count,
		Entries:   gashaponOutcomesToGame(gashaponParseOutcomes(rec.EntriesJson)),
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
}

type LockGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// countOfRequest 幂等/屏障跳过路径：按 requestId 读参与记录如实返回；未找到 → 回退请求值
func (l *LockGashaponLogic) countOfRequest(db *gorm.DB, configId int64, requestId string, fallback int64) int64 {
	var st gamemodelnc.GashaponState
	if err := db.Where("config_id = ?", configId).First(&st).Error; err != nil {
		return fallback
	}
	state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
	if err != nil {
		return fallback
	}
	for _, r := range state.Records {
		if r.RequestID == requestId {
			return r.Count
		}
	}
	return fallback
}

type ReleaseGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// errGashaponRecordDup 事务内哨兵：同 requestId 的扭蛋记录已存在（另一 gid 已结算）
var errGashaponRecordDup = errors.New("gashapon record duplicated")

type SettleGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// filterGashaponPool 候选池同步：抽中的条目余量归零后移出（其余原样保留）
func filterGashaponPool(pool []core.PrizeEntry, pickedId int64, remainingByName map[int64]int64) []core.PrizeEntry {
	if remainingByName[pickedId] > 0 {
		return pool
	}
	out := make([]core.PrizeEntry, 0, len(pool))
	for _, p := range pool {
		if p.Id == pickedId {
			continue
		}
		out = append(out, p)
	}
	return out
}

// gashaponOutcomesToGame regular 奖结果 → 响应条目
func gashaponOutcomesToGame(outcomes []gashaponOutcome) []*play.GashaponEntryOutcome {
	items := make([]*play.GashaponEntryOutcome, 0, len(outcomes))
	for _, o := range outcomes {
		items = append(items, &play.GashaponEntryOutcome{Name: o.Name, IsPrize: o.IsPrize})
	}
	return items
}

// specialAwardItems 特别奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleGashaponLogic) specialAwardItems(awards []core.GashaponAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		item := &play.PrizeItem{Kind: a.PrizeKind, Points: a.PointsAmount, CouponTemplateNo: a.CouponTemplateNo}
		switch a.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(a.CashFen)
		case prizeCoupon:
			item.CouponName = couponNameOf(l.ctx, l.svcCtx, a.CouponTemplateNo)
		}
		items = append(items, item)
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 特别奖 + 当前状态如实返回
func (l *SettleGashaponLogic) readBackRecord(requestId string, userId int64) (*play.SettleGashaponResp, error) {
	var rec gamemodelnc.GashaponRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "扭蛋记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var special []core.GashaponAward
	var st gamemodelnc.GashaponState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == gashaponStateSettled
		if state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson); err == nil {
			special = gashaponAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, special: %d", requestId, rec.Id, settled, len(special))
	return &play.SettleGashaponResp{
		RecordId:      rec.Id,
		RegularPrizes: gashaponOutcomesToGame(gashaponParseOutcomes(rec.EntriesJson)),
		SpecialAwards: l.specialAwardItems(special),
		Settled:       settled,
		Status:        "SETTLED",
	}, nil
}

type SettleGashaponRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}
