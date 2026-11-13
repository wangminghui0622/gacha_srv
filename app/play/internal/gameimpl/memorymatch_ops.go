package gameimpl

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ApplyMemoryMatchDestinyDrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type ApplyMemoryMatchPairLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type CompleteMemoryMatchRoundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func strPtr(s string) *string { return &s }

type GetMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GetMemoryMatchConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GetMemoryMatchRoundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GetMemoryMatchSessionByRequestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GetMemoryMatchSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantMemoryMatchPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantMemoryMatchPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type LockMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type LockMemoryMatchRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

const (
	mmSessionPending         int64 = 1
	mmSessionAwaitingDestiny int64 = 2
	mmSessionAwaitingSecond  int64 = 3
	mmSessionPlaying         int64 = 4
	mmSessionSettling        int64 = 5 // 终局结算中（本仓单事务内瞬态，落库直接 COMPLETED）
	mmSessionCompleted       int64 = 6
	mmSessionExpired         int64 = 7

	mmRoundReady    int64 = 1
	mmRoundPlaying  int64 = 2
	mmRoundResolved int64 = 3
	mmRoundSkipped  int64 = 4

	mmSettlementPending int64 = 1 // 待发奖
	mmSettlementGranted int64 = 2 // 已发奖

	mmPayoutPending int64 = 1
	mmPayoutPaid    int64 = 2
)

// mmRules 规则快照（sessions.rules_json；购买时配置原样，之后配置变更不影响已购会话）
type mmRules struct {
	RoundsPerPurchase     int64   `json:"rounds_per_purchase"`
	MaxPairCount          int64   `json:"max_pair_count"`
	PurchasePriceMinor    int64   `json:"purchase_price_minor"`
	WalletKind            int64   `json:"wallet_kind"`
	SecondDestinyEnabled  bool    `json:"second_destiny_enabled"`
	DeckStrategy          int64   `json:"deck_strategy"`
	PatternWeights        []int64 `json:"pattern_weights"`
	ExactMinPairCount     int64   `json:"exact_min_pair_count"`
	ExactMaxPairCount     int64   `json:"exact_max_pair_count"`
	ExactAttemptLimit     int64   `json:"exact_attempt_limit"`
	ChampionMinTotalPairs int64   `json:"champion_min_total_pairs"`
	ChampionShareBps      int64   `json:"champion_share_bps"`
}

// mmAppearance 外观快照（sessions.appearance_json）
type mmAppearance struct {
	FacePresetID int64 `json:"face_preset_id"`
	BackPresetID int64 `json:"back_preset_id"`
}

// mmReward 结算奖励快照（settlements.reward_json）
type mmReward struct {
	TotalPairCount int64         `json:"total_pair_count"`
	HasPrize       bool          `json:"has_prize"`
	PrizeKind      int64         `json:"prize_kind"` // 1 现金 2 积分 3 券；HasPrize=false 无
	CashFen        int64         `json:"cash_fen"`
	PointsAmount   int64         `json:"points_amount"`
	CouponTemplate string        `json:"coupon_template_no"`
	SkuId          int64         `json:"sku_id"`
	EntryName      string        `json:"entry_name"`
	EntryID        int64         `json:"entry_id"`
	Remaining      int64         `json:"remaining"`
	RoundPairs     map[int64]int `json:"round_pairs"`
}

// mmParseRules 解析规则快照；非法 → 3020
func mmParseRules(raw string) (*mmRules, error) {
	var r mmRules
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	if r.RoundsPerPurchase < 1 || r.MaxPairCount < 1 || r.PurchasePriceMinor <= 0 {
		return nil, errors.New("rules snapshot invalid")
	}
	return &r, nil
}

func mmRulesJSON(r *mmRules) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// mmEngineSeed 购买时 crypto/rand 32B（hex 落库）；hex 解码回字节
func mmEngineSeedHex() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func mmEngineSeedBytes(seedHex string) ([]byte, error) {
	b, err := hex.DecodeString(seedHex)
	if err != nil || len(b) != 32 {
		return nil, errors.New("engine seed invalid")
	}
	return b, nil
}

// mmCodeOrDBError 事务错误还原：已是业务 CodeError 原样返回；其余按数据库错误包装
func mmCodeOrDBError(err error) error {
	var ce *errorx.CodeError
	if errors.As(err, &ce) {
		return err
	}
	return errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
}

// mmAborted 业务 CodeError → SAGA Aborted（触发补偿）；未知错误按 500 Aborted（不吞成重试风暴）
func mmAborted(err error) error {
	var ce *errorx.CodeError
	if errors.As(err, &ce) {
		return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
	}
	return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", errorx.CodeServerError, "内部错误"))
}

// mmMapEngineErr 引擎哨兵错误 → errorx（玩法 RPC 非 SAGA 分支：业务码经拦截器转 gRPC）
func mmMapEngineErr(err error) error {
	switch {
	case errors.Is(err, memorymatch.ErrRoundFinished):
		return errorx.NewCodeError(errorx.CodeMemoryMatchRoundFinished, "回合已结束")
	case errors.Is(err, memorymatch.ErrRoundNotFinished):
		return errorx.NewCodeError(errorx.CodeMemoryMatchRoundNotFinished, "回合未完成")
	case errors.Is(err, memorymatch.ErrDestinyUnavailable):
		return errorx.NewCodeError(errorx.CodeMemoryMatchDestinyUnavailable, "命运抽牌不可用")
	case errors.Is(err, memorymatch.ErrInvalidPair):
		return errorx.NewCodeError(errorx.CodeParamError, "配对无效")
	case errors.Is(err, memorymatch.ErrStateInvalid),
		errors.Is(err, memorymatch.ErrOpeningInvalid),
		errors.Is(err, memorymatch.ErrActionsLimit):
		return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话状态冲突")
	case errors.Is(err, memorymatch.ErrConfigInvalid):
		return errorx.NewCodeError(errorx.CodeGameConfigInvalid, "玩法配置无效")
	default:
		return errorx.NewCodeError(errorx.CodeServerError, err.Error())
	}
}

func int32SliceToInt64(in []int32) []int64 {
	out := make([]int64, len(in))
	for i, v := range in {
		out[i] = int64(v)
	}
	return out
}

// mmPrizeItem 奖档 → PrizeItem（谢谢参与 kind=0）
func mmPrizeItem(kind, cashFen, points int64, couponTemplateNo, name string, skuId int64) *play.PrizeItem {
	switch kind {
	case prizeCash:
		return &play.PrizeItem{Kind: 1, Amount: utils.FenToYuan(cashFen)}
	case prizePoints:
		return &play.PrizeItem{Kind: 2, Points: points}
	case prizeCoupon:
		return &play.PrizeItem{Kind: 3, CouponTemplateNo: couponTemplateNo, CouponName: name}
	case prizeGoods:
		return &play.PrizeItem{Kind: prizeGoods, SkuId: skuId, ItemName: name}
	default:
		return &play.PrizeItem{Kind: 0}
	}
}

func mmFacePresets(cfg *gamemodel.MemoryMatchConfigs) ([]memorymatch.FacePreset, error) {
	var presets []memorymatch.FacePreset
	if err := json.Unmarshal([]byte(cfg.FacePresetsJson), &presets); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "正面预设配置无效")
	}
	return presets, nil
}

func mmBackPresets(cfg *gamemodel.MemoryMatchConfigs) ([]memorymatch.BackPreset, error) {
	var presets []memorymatch.BackPreset
	if err := json.Unmarshal([]byte(cfg.BackPresetsJson), &presets); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "背面预设配置无效")
	}
	return presets, nil
}

// mmRoundResult 回合结果解析结构（result_json）
type mmRoundResult struct {
	PairCount int `json:"pair_count"`
}

type SelectMemoryMatchDestinyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type SelectMemoryMatchSecondDestinyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type SetMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type SettleMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type SettleMemoryMatchRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type SkipMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}
