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
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// 连击链状态（combo_chain_state.status）
const (
	comboChainStateOpen    int64 = 1
	comboChainStateSettled int64 = 2
)

// 连击链记录状态（combo_chain_records.status）
const (
	comboChainRecordNormal   int64 = 1
	comboChainRecordCanceled int64 = 2
)

// comboChainStateJSON state 行四段 JSON 内存视图
type comboChainStateJSON struct {
	Participants  []int64                         `json:"participants"`
	Awards        []core.ComboChainAward          `json:"awards"`
	TierRemaining []int32                         `json:"tier_remaining"`
	StreaksBefore map[int64]core.ComboChainStreak `json:"streaks_before"`
}

// comboChainParseState state 行四段 JSON → 内存状态（空串取零值；坏 JSON → 错误）
func comboChainParseState(participantsJson, awardsJson, tierRemainingJson, streaksBeforeJson string) (*comboChainStateJSON, error) {
	state := &comboChainStateJSON{}
	if participantsJson != "" {
		if err := json.Unmarshal([]byte(participantsJson), &state.Participants); err != nil {
			return nil, err
		}
	}
	if state.Participants == nil {
		state.Participants = []int64{}
	}
	if awardsJson != "" {
		if err := json.Unmarshal([]byte(awardsJson), &state.Awards); err != nil {
			return nil, err
		}
	}
	if state.Awards == nil {
		state.Awards = []core.ComboChainAward{}
	}
	if tierRemainingJson != "" {
		if err := json.Unmarshal([]byte(tierRemainingJson), &state.TierRemaining); err != nil {
			return nil, err
		}
	}
	if state.TierRemaining == nil {
		state.TierRemaining = []int32{}
	}
	if streaksBeforeJson != "" {
		if err := json.Unmarshal([]byte(streaksBeforeJson), &state.StreaksBefore); err != nil {
			return nil, err
		}
	}
	if state.StreaksBefore == nil {
		state.StreaksBefore = map[int64]core.ComboChainStreak{}
	}
	return state, nil
}

// comboChainParseSlots global_slots_json → 全局奖槽（空/坏 JSON → 错误，调用方报 3020）
func comboChainParseSlots(jsonStr string) ([]core.ComboChainGlobalSlot, error) {
	if jsonStr == "" {
		return nil, errors.New("empty global slots json")
	}
	var slots []core.ComboChainGlobalSlot
	if err := json.Unmarshal([]byte(jsonStr), &slots); err != nil {
		return nil, err
	}
	return slots, nil
}

// comboChainParseShared shared_prizes_json → 共享奖配置（空/坏 JSON → 错误，调用方报 3020）
func comboChainParseShared(jsonStr string) ([]core.ComboChainSharedPrize, error) {
	if jsonStr == "" {
		return nil, errors.New("empty shared prizes json")
	}
	var prizes []core.ComboChainSharedPrize
	if err := json.Unmarshal([]byte(jsonStr), &prizes); err != nil {
		return nil, err
	}
	return prizes, nil
}

// comboChainParticipantsJSON 参与者列表 → JSON（[]int64 恒可编码）
func comboChainParticipantsJSON(participants []int64) string {
	b, _ := json.Marshal(participants)
	return string(b)
}

// comboChainAwardsJSON 结算奖列表 → JSON（[]struct 恒可编码）
func comboChainAwardsJSON(awards []core.ComboChainAward) string {
	b, _ := json.Marshal(awards)
	return string(b)
}

// comboChainTierRemainingJSON 档位库存 → JSON（[]int32 恒可编码）
func comboChainTierRemainingJSON(remaining []int32) string {
	b, _ := json.Marshal(remaining)
	return string(b)
}

// comboChainStreaksBeforeJSON 连击快照 → JSON（map 恒可编码）
func comboChainStreaksBeforeJSON(streaks map[int64]core.ComboChainStreak) string {
	b, _ := json.Marshal(streaks)
	return string(b)
}

// comboChainAwardsOfRequest 取指定 requestId 归属的结算奖（06 发放本 gid 触发的结算奖）
func comboChainAwardsOfRequest(awards []core.ComboChainAward, requestId string) []core.ComboChainAward {
	out := make([]core.ComboChainAward, 0, len(awards))
	for _, a := range awards {
		if a.RequestID == requestId {
			out = append(out, a)
		}
	}
	return out
}

// loadComboChainStreaks 事务内读连击计数：userId → 连击值
func loadComboChainStreaks(db *gorm.DB, configId int64) (map[int64]core.ComboChainStreak, error) {
	var rows []gamemodelnc.ComboChainStreaks
	if err := db.Where("config_id = ?", configId).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]core.ComboChainStreak, len(rows))
	for _, r := range rows {
		out[r.UserId] = core.ComboChainStreak{EmptyStreak: int32(r.EmptyStreak), WinStreak: int32(r.WinStreak)}
	}
	return out, nil
}

// upsertComboChainStreak 连击计数落库（存在则更新，缺失则插入；快照回滚与结算推进共用）
func upsertComboChainStreak(db *gorm.DB, configId, userId int64, s core.ComboChainStreak) error {
	var row gamemodelnc.ComboChainStreaks
	err := db.Where("config_id = ? AND user_id = ?", configId, userId).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&gamemodelnc.ComboChainStreaks{
			ConfigId:    configId,
			UserId:      userId,
			EmptyStreak: int64(s.EmptyStreak),
			WinStreak:   int64(s.WinStreak),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&gamemodelnc.ComboChainStreaks{}).Where("id = ?", row.Id).
		Updates(map[string]interface{}{
			"empty_streak": s.EmptyStreak,
			"win_streak":   s.WinStreak,
			"updated_at":   time.Now(),
		}).Error
}

// prizeShapeItem 资产形状 → PrizeItem（现金金额转元；券名尽力解析，展示用）
func prizeShapeItem(ctx context.Context, svcCtx *svc.ServiceContext, shape core.PrizeShape) *play.PrizeItem {
	item := &play.PrizeItem{Kind: shape.PrizeKind, Points: shape.PointsAmount, CouponTemplateNo: shape.CouponTemplateNo, SkuId: shape.SkuId, ItemName: shape.ItemName}
	switch shape.PrizeKind {
	case prizeCash:
		item.Amount = utils.FenToYuan(shape.CashFen)
	case prizeCoupon:
		item.CouponName = couponNameOf(ctx, svcCtx, shape.CouponTemplateNo)
	}
	return item
}

type GetComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantComboChainPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantComboChainPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type ListComboChainRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// buildItem 记录行 → 响应条目（金额转元；结算奖按 requestId 过滤自 state.awards_json）
func (l *ListComboChainRecordsLogic) buildItem(rec *gamemodelnc.ComboChainRecords) *play.ComboChainRecordItem {
	item := &play.ComboChainRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		CarIndex:  rec.CarIndex,
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		if state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson); err == nil {
			for _, a := range comboChainAwardsOfRequest(state.Awards, rec.RequestId) {
				item.Awards = append(item.Awards, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
			}
		}
	}
	return item
}

type LockComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type ReleaseComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// errComboChainRecordDup 事务内哨兵：同 requestId 的连击链记录已存在（另一 gid 已结算）
var errComboChainRecordDup = errors.New("combo chain record duplicated")

type SettleComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// comboChainAwardItems 结算奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleComboChainLogic) comboChainAwardItems(awards []core.ComboChainAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		items = append(items, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 结算奖 + 当前状态如实返回
func (l *SettleComboChainLogic) readBackRecord(requestId string, userId int64) (*play.SettleComboChainResp, error) {
	var rec gamemodelnc.ComboChainRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "连击链记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var awards []core.ComboChainAward
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == comboChainStateSettled
		if state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson); err == nil {
			awards = comboChainAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, awards: %d", requestId, rec.Id, settled, len(awards))
	return &play.SettleComboChainResp{
		RecordId: rec.Id,
		Awards:   l.comboChainAwardItems(awards),
		Settled:  settled,
		Status:   "SETTLED",
	}, nil
}

type SettleComboChainRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}
