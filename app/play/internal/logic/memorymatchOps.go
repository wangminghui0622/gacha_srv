package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/money/moneyClient"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── M15b 记忆翻牌共享：会话/回合状态常量、配置快照、回合状态 JSON、终局结算、发奖、擂主 ──

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

func mmParseAppearance(raw string) (*mmAppearance, error) {
	var a mmAppearance
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// mmParseRoundState 回合进度 JSON → 引擎状态
func mmParseRoundState(raw string) (*memorymatch.RoundState, error) {
	var st memorymatch.RoundState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func mmRoundStateJSON(st *memorymatch.RoundState) string {
	b, _ := json.Marshal(st)
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

// mmLoadSession 会话按 id 加载 + 归属校验（非本人 → 404，防枚举）
func mmLoadSession(db *gorm.DB, sessionID, userID int64) (*gamemodelnc.MemoryMatchSessions, error) {
	var sess gamemodelnc.MemoryMatchSessions
	if err := db.Where("id = ? AND user_id = ?", sessionID, userID).First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	return &sess, nil
}

// mmSessionRules 会话规则快照解析（非法 → 3039 冲突，数据异常）
func mmSessionRules(sess *gamemodelnc.MemoryMatchSessions) (*mmRules, error) {
	rules, err := mmParseRules(sess.RulesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话规则快照异常")
	}
	return rules, nil
}

// mmRoundView 回合行 → 响应视图（现场派生校验：状态非法 → 3039）
func mmRoundView(sess *gamemodelnc.MemoryMatchSessions, rules *mmRules, round *gamemodelnc.MemoryMatchRounds, seedHex string) (*play.GetMemoryMatchRoundResp, error) {
	st, err := mmParseRoundState(round.StateJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态异常")
	}
	engineSeed, err := mmEngineSeedBytes(seedHex)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "引擎种子异常")
	}
	roundSeed := memorymatch.RoundSeed(engineSeed, sess.Id, round.RoundNo)
	if err := memorymatch.ValidateOpening(st, roundSeed, int(rules.MaxPairCount)); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合开局校验失败")
	}
	hand, err := st.Hand()
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态异常")
	}
	view := &play.GetMemoryMatchRoundResp{
		RoundNo:                round.RoundNo,
		State:                  play.MemoryMatchRoundState(round.State),
		HandCardIds:            append([]int64(nil), int32SliceToInt64(hand)...),
		DrawPosition:           int64(st.DrawPosition),
		PairCount:              int64(st.PairCount),
		MaxPairCount:           rules.MaxPairCount,
		AvailableDestiny:       int64(st.AvailableDestiny),
		DestinyDraws:           int64(st.DestinyDraws),
		SeedCommitment:         st.SeedCommitment,
		Actions:                st.Actions,
		Terminal:               st.IsTerminal(int(rules.MaxPairCount)),
		DestinyPatternId:       int32(sess.DestinyPatternId),
		SecondDestinyPatternId: int32(sess.SecondDestinyPatternId),
	}
	if round.ResultHash != nil && *round.ResultHash != "" {
		view.ResultHash = *round.ResultHash
	}
	return view, nil
}

func int32SliceToInt64(in []int32) []int64 {
	out := make([]int64, len(in))
	for i, v := range in {
		out[i] = int64(v)
	}
	return out
}

// mmGrantOne 单奖幂等发放（grant_no 全 uk 幂等；跨库钱包走 money-rpc，M11 同款）
func mmGrantOne(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, grantNo string, prizeKind, cashFen, points int64, couponTemplateNo string) error {
	switch prizeKind {
	case prizeCash:
		if _, err := svcCtx.MoneyRpc.CreditWallet(ctx, &moneyClient.CreditWalletReq{
			UserId:    userID,
			Kind:      walletCash,
			Amount:    utils.FenToYuan(cashFen),
			RequestId: grantNo,
		}); err != nil {
			return err
		}
	case prizePoints:
		if _, err := svcCtx.MoneyRpc.CreditWallet(ctx, &moneyClient.CreditWalletReq{
			UserId:    userID,
			Kind:      walletLoyaltyPoint,
			Amount:    utils.FenToYuan(points),
			RequestId: grantNo,
		}); err != nil {
			return err
		}
	case prizeCoupon:
		// 券经营销 RPC 发放（coupon_no="uc_"+grantNo 撞键幂等；旧同服务直插 user_coupons 表）
		if _, err := svcCtx.MarketingRpc.GrantPlayCoupon(ctx, &marketingClient.GrantPlayCouponReq{
			RequestId:  grantNo,
			UserId:     userID,
			TemplateNo: couponTemplateNo,
		}); err != nil {
			return err
		}
	default:
		// kind=0（谢谢参与）no-op
		return nil
	}
	return nil
}

// mmRetryPendingSettlementGrant 补发 PENDING 结算奖（幂等 grant_no="mmr:<sessionID>"；重放 CompleteRound/Skip 时调用）
func mmRetryPendingSettlementGrant(ctx context.Context, svcCtx *svc.ServiceContext, sess *gamemodelnc.MemoryMatchSessions) error {
	var stl gamemodelnc.MemoryMatchSettlements
	if err := svcCtx.DB.Where("session_id = ?", sess.Id).First(&stl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if stl.Status != mmSettlementPending {
		return nil
	}
	var reward mmReward
	if err := json.Unmarshal([]byte(stl.RewardJson), &reward); err != nil {
		return err
	}
	if !reward.HasPrize {
		// 无奖：直接置已发
		return svcCtx.DB.Model(&gamemodelnc.MemoryMatchSettlements{}).
			Where("session_id = ? AND status = ?", sess.Id, mmSettlementPending).
			Updates(map[string]interface{}{"status": mmSettlementGranted, "updated_at": time.Now()}).Error
	}
	grantNo := fmt.Sprintf("mmr:%d", sess.Id)
	if err := mmGrantOne(ctx, svcCtx, sess.UserId, grantNo, reward.PrizeKind, reward.CashFen, reward.PointsAmount, reward.CouponTemplate); err != nil {
		return err
	}
	return svcCtx.DB.Model(&gamemodelnc.MemoryMatchSettlements{}).
		Where("session_id = ? AND status = ?", sess.Id, mmSettlementPending).
		Updates(map[string]interface{}{"status": mmSettlementGranted, "updated_at": time.Now()}).Error
}

// mmRetryChampionPayouts 补发该配置 PENDING 擂主打款（grant_no 幂等；后续购买/结算时兜底）
func mmRetryChampionPayouts(ctx context.Context, svcCtx *svc.ServiceContext, configID int64) error {
	var rows []gamemodelnc.MemoryMatchChampionPayouts
	if err := svcCtx.DB.Where("config_id = ? AND status = ?", configID, mmPayoutPending).Find(&rows).Error; err != nil {
		return err
	}
	for _, p := range rows {
		if err := mmGrantOne(ctx, svcCtx, p.UserId, p.GrantNo, prizeCash, p.AmountMinor, 0, ""); err != nil {
			return err
		}
		if err := svcCtx.DB.Model(&gamemodelnc.MemoryMatchChampionPayouts{}).
			Where("grant_no = ? AND status = ?", p.GrantNo, mmPayoutPending).
			Updates(map[string]interface{}{"status": mmPayoutPaid, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
	}
	return nil
}

// mmSettlementView 结算行 → 响应视图
func mmSettlementView(stl *gamemodelnc.MemoryMatchSettlements) (*play.MemoryMatchSettlementView, error) {
	var reward mmReward
	if err := json.Unmarshal([]byte(stl.RewardJson), &reward); err != nil {
		return nil, err
	}
	view := &play.MemoryMatchSettlementView{
		TotalPairCount: reward.TotalPairCount,
		FinalHash:      stl.ResultHash,
		Status:         stl.Status,
	}
	if reward.HasPrize {
		view.Prize = mmPrizeItem(reward.PrizeKind, reward.CashFen, reward.PointsAmount, reward.CouponTemplate, reward.EntryName)
	} else {
		view.Prize = &play.PrizeItem{Kind: 0}
	}
	return view, nil
}

// mmPrizeItem 奖档 → PrizeItem（谢谢参与 kind=0）
func mmPrizeItem(kind, cashFen, points int64, couponTemplateNo, name string) *play.PrizeItem {
	switch kind {
	case prizeCash:
		return &play.PrizeItem{Kind: 1, Amount: utils.FenToYuan(cashFen)}
	case prizePoints:
		return &play.PrizeItem{Kind: 2, Points: points}
	case prizeCoupon:
		return &play.PrizeItem{Kind: 3, CouponTemplateNo: couponTemplateNo, CouponName: name}
	default:
		return &play.PrizeItem{Kind: 0}
	}
}

// mmEntryOfConfig 该配置奖档列表（结算时按 total_pair_count 精确匹配）
func mmEntryOfConfig(db *gorm.DB, configID int64) ([]gamemodel.MemoryMatchEntries, error) {
	var entries []gamemodel.MemoryMatchEntries
	if err := db.Where("config_id = ?", configID).Order("total_pair_count").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// mmConfigToRules 配置行 → 规则快照（含配置校验；非法 → 3020）
func mmConfigToRules(cfg *gamemodel.MemoryMatchConfigs) (*mmRules, error) {
	var weights []int64
	if err := json.Unmarshal([]byte(cfg.PatternWeightsJson), &weights); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "发牌权重配置无效")
	}
	var faces []memorymatch.FacePreset
	if err := json.Unmarshal([]byte(cfg.FacePresetsJson), &faces); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "正面预设配置无效")
	}
	var backs []memorymatch.BackPreset
	if err := json.Unmarshal([]byte(cfg.BackPresetsJson), &backs); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "背面预设配置无效")
	}
	rules := &mmRules{
		RoundsPerPurchase:     cfg.RoundsPerPurchase,
		MaxPairCount:          cfg.MaxPairCount,
		PurchasePriceMinor:    cfg.PurchasePriceMinor,
		WalletKind:            cfg.WalletKind,
		SecondDestinyEnabled:  cfg.SecondDestinyEnabled == 1,
		DeckStrategy:          cfg.DeckStrategy,
		PatternWeights:        weights,
		ExactMinPairCount:     cfg.ExactMinPairCount,
		ExactMaxPairCount:     cfg.ExactMaxPairCount,
		ExactAttemptLimit:     cfg.ExactAttemptLimit,
		ChampionMinTotalPairs: cfg.ChampionMinTotalPairs,
		ChampionShareBps:      cfg.ChampionShareBps,
	}
	if err := memorymatch.ValidateConfigRules(int(rules.RoundsPerPurchase), int(rules.MaxPairCount),
		rules.SecondDestinyEnabled, int32(rules.DeckStrategy), rules.PatternWeights,
		int(rules.ExactMinPairCount), int(rules.ExactMaxPairCount), int(rules.ExactAttemptLimit),
		faces, backs, int(cfg.DefaultFacePresetId), int(cfg.DefaultBackPresetId),
		int(rules.ChampionMinTotalPairs), int(rules.ChampionShareBps)); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "玩法配置校验失败")
	}
	return rules, nil
}

// mmLoadConfig 配置按 id 加载（缓存模型）；不存在/下架 → 404；JSON/规则非法 → 3020
func mmLoadConfig(db *gorm.DB, configID int64) (*gamemodel.MemoryMatchConfigs, *mmRules, error) {
	var cfg gamemodel.MemoryMatchConfigs
	if err := db.Where("id = ?", configID).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errorx.NewCodeError(errorx.CodeNotFound, "玩法不存在")
		}
		return nil, nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	if cfg.Status != 1 {
		return nil, nil, errorx.NewCodeError(errorx.CodeNotFound, "玩法不存在")
	}
	rules, err := mmConfigToRules(&cfg)
	if err != nil {
		return nil, nil, err
	}
	return &cfg, rules, nil
}

// ── 视图构建（配置/擂主/外观）──

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

// mmConfigItem 配置 + 奖档 → 响应视图
func mmConfigItem(cfg *gamemodel.MemoryMatchConfigs, rules *mmRules, entries []gamemodel.MemoryMatchEntries) (*play.MemoryMatchConfigItem, error) {
	faces, err := mmFacePresets(cfg)
	if err != nil {
		return nil, err
	}
	backs, err := mmBackPresets(cfg)
	if err != nil {
		return nil, err
	}
	item := &play.MemoryMatchConfigItem{
		ConfigId:              cfg.Id,
		Name:                  cfg.Name,
		RoundsPerPurchase:     rules.RoundsPerPurchase,
		MaxPairCount:          rules.MaxPairCount,
		PurchasePrice:         utils.FenToYuan(rules.PurchasePriceMinor),
		WalletKind:            rules.WalletKind,
		SecondDestinyEnabled:  rules.SecondDestinyEnabled,
		DeckStrategy:          play.MemoryMatchDeckStrategy(rules.DeckStrategy),
		PatternWeights:        rules.PatternWeights,
		ExactMinPairCount:     rules.ExactMinPairCount,
		ExactMaxPairCount:     rules.ExactMaxPairCount,
		ExactAttemptLimit:     rules.ExactAttemptLimit,
		DefaultFacePresetId:   cfg.DefaultFacePresetId,
		DefaultBackPresetId:   cfg.DefaultBackPresetId,
		ChampionMinTotalPairs: rules.ChampionMinTotalPairs,
		ChampionShareBps:      rules.ChampionShareBps,
		Status:                cfg.Status,
	}
	for _, f := range faces {
		fi := &play.MemoryMatchFacePreset{PresetId: f.PresetID, Name: f.Name}
		for _, face := range f.Faces {
			fi.Faces = append(fi.Faces, &play.MemoryMatchFaceItem{
				PatternId: face.PatternID, Name: face.Name, ImageUrl: face.ImageURL,
			})
		}
		item.FacePresets = append(item.FacePresets, fi)
	}
	for _, b := range backs {
		item.BackPresets = append(item.BackPresets, &play.MemoryMatchBackPreset{
			PresetId: b.PresetID, Name: b.Name,
			BackImageUrl: b.BackImageURL, FaceBackgroundImageUrl: b.FaceBackgroundImageURL,
		})
	}
	for _, e := range entries {
		item.Entries = append(item.Entries, &play.MemoryMatchEntryItem{
			EntryId:        e.Id,
			Name:           e.Name,
			TotalPairCount: e.TotalPairCount,
			Prize:          mmPrizeItem(e.PrizeKind, e.CashFen, e.PointsAmount, e.CouponTemplateNo, e.Name),
			Remaining:      e.Remaining,
		})
	}
	return item, nil
}

// mmChampionView 现任擂主视图（无擂主 → exists=false）
func mmChampionView(db *gorm.DB, configID int64) (*play.MemoryMatchChampionInfo, error) {
	info := &play.MemoryMatchChampionInfo{}
	var ch gamemodelnc.MemoryMatchChampion
	if err := db.Where("config_id = ?", configID).First(&ch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return info, nil
		}
		return nil, err
	}
	info.Exists = true
	info.UserId = ch.UserId
	info.TotalPairCount = ch.TotalPairCount
	info.PendingIncome = utils.FenToYuan(ch.PendingIncomeMinor)
	return info, nil
}

// mmAppearanceView 当前选择（已存或默认）+ 已拥有正面预设（权益行存在即拥有；背面不校验权益）
func mmAppearanceView(db *gorm.DB, cfg *gamemodel.MemoryMatchConfigs, userID int64) (*play.MemoryMatchAppearanceView, error) {
	view := &play.MemoryMatchAppearanceView{
		FacePresetId: cfg.DefaultFacePresetId,
		BackPresetId: cfg.DefaultBackPresetId,
	}
	if userID > 0 {
		var sel gamemodelnc.MemoryMatchAppearanceSelections
		if err := db.Where("config_id = ? AND user_id = ?", cfg.Id, userID).First(&sel).Error; err == nil {
			view.FacePresetId = sel.FacePresetId
			view.BackPresetId = sel.BackPresetId
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		var ents []gamemodelnc.MemoryMatchAppearanceEntitlements
		if err := db.Where("config_id = ? AND user_id = ?", cfg.Id, userID).Find(&ents).Error; err != nil {
			return nil, err
		}
		for _, e := range ents {
			view.OwnedFacePresetIds = append(view.OwnedFacePresetIds, e.PresetId)
		}
	}
	return view, nil
}

// mmCreateRounds 选命运后一次性创建 1..purchased 回合开局（引擎种子须已落库；回合创建即 PLAYING）
func mmCreateRounds(db *gorm.DB, sess *gamemodelnc.MemoryMatchSessions, rules *mmRules) error {
	if sess.EngineSeed == nil || *sess.EngineSeed == "" {
		return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "引擎种子异常")
	}
	engineSeed, err := mmEngineSeedBytes(*sess.EngineSeed)
	if err != nil {
		return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "引擎种子异常")
	}
	for roundNo := int64(1); roundNo <= rules.RoundsPerPurchase; roundNo++ {
		roundSeed := memorymatch.RoundSeed(engineSeed, sess.Id, roundNo)
		st := memorymatch.Opening(roundSeed, int32(rules.DeckStrategy), rules.PatternWeights,
			int(rules.ExactMinPairCount), int(rules.ExactMaxPairCount), int(rules.ExactAttemptLimit))
		row := &gamemodelnc.MemoryMatchRounds{
			SessionId: sess.Id,
			RoundNo:   roundNo,
			State:     mmRoundPlaying,
			StateJson: mmRoundStateJSON(st),
			PairCount: 0,
			Version:   1,
		}
		if err := db.Create(row).Error; err != nil {
			return err
		}
	}
	return nil
}

// ── 终局结算（CompleteRound/Skip 事务内共用）──

// mmRoundResult 回合结果解析结构（result_json）
type mmRoundResult struct {
	PairCount int `json:"pair_count"`
}

// mmVerifyRoundResult 逐回合 Verify（老项目同款）：result_json 解析 → pair_count 与状态一致 →
// 重算 result_hash 与落库一致（防篡改哈希链）
func mmVerifyRoundResult(round *gamemodelnc.MemoryMatchRounds) error {
	if round.State != mmRoundResolved && round.State != mmRoundSkipped {
		return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "存在未完成回合")
	}
	if round.ResultJson == nil || round.ResultHash == nil {
		return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合结果缺失")
	}
	var res mmRoundResult
	if err := json.Unmarshal([]byte(*round.ResultJson), &res); err != nil {
		return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合结果异常")
	}
	jsonBytes, hashBytes := memorymatch.RoundResult(res.PairCount)
	if string(jsonBytes) != *round.ResultJson || hex.EncodeToString(hashBytes) != *round.ResultHash {
		return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合结果校验失败")
	}
	if int64(res.PairCount) != round.PairCount {
		return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合配对计数不一致")
	}
	return nil
}

// mmFinalSettlement 终局结算（调用方须已 FOR UPDATE 会话行；completed==purchased）：
//
//	① 1..purchased 回合全 RESOLVED/SKIPPED 且逐回合 Verify；
//	② total=Σpair_count、final_hash=SHA256("memory-match/v1/final"‖r1..rN)；
//	③ entries 按 total 精确匹配：有奖档 FOR UPDATE 扣 remaining（0 → 3045 售罄整事务回滚）；
//	④ 插 settlement(status=1 待发奖) + 会话 COMPLETED + 擂主换擂（事件 uk session_id 幂等、
//	   现任 pending → PENDING 打款流水 grant_no=mm-champion:<本会话ID>，事务外幂等补发）。
//
// 发奖在事务外（跨库钱包），mmRetryPendingSettlementGrant/mmRetryChampionPayouts 兜底。
func mmFinalSettlement(db *gorm.DB, sess *gamemodelnc.MemoryMatchSessions, rules *mmRules) (*gamemodelnc.MemoryMatchSettlements, int64, error) {
	var rounds []gamemodelnc.MemoryMatchRounds
	if err := db.Where("session_id = ?", sess.Id).Order("round_no ASC").Find(&rounds).Error; err != nil {
		return nil, 0, err
	}
	if int64(len(rounds)) != rules.RoundsPerPurchase {
		return nil, 0, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合数不完整")
	}
	total := int64(0)
	hashes := make([][]byte, 0, len(rounds))
	roundPairs := make(map[int64]int, len(rounds))
	for i := range rounds {
		if err := mmVerifyRoundResult(&rounds[i]); err != nil {
			return nil, 0, err
		}
		hashBytes, err := hex.DecodeString(*rounds[i].ResultHash)
		if err != nil || len(hashBytes) != 32 {
			return nil, 0, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合结果哈希异常")
		}
		total += rounds[i].PairCount
		hashes = append(hashes, hashBytes)
		roundPairs[rounds[i].RoundNo] = int(rounds[i].PairCount)
	}
	finalHash := hex.EncodeToString(memorymatch.FinalHash(hashes))

	// ③ 奖档精确匹配（uk(config_id,total_pair_count) 唯一）
	entries, err := mmEntryOfConfig(db, sess.ConfigId)
	if err != nil {
		return nil, 0, err
	}
	var matched *gamemodel.MemoryMatchEntries
	for i := range entries {
		if entries[i].TotalPairCount == total {
			matched = &entries[i]
			break
		}
	}
	reward := mmReward{TotalPairCount: total, RoundPairs: roundPairs}
	if matched != nil && matched.IsPrize == 1 && matched.PrizeKind > 0 {
		// 有奖档：FOR UPDATE 扣库存（-1 不限；0 → 3045 售罄）
		var entry gamemodel.MemoryMatchEntries
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", matched.Id).First(&entry).Error; err != nil {
			return nil, 0, err
		}
		if entry.Remaining == 0 {
			return nil, 0, errorx.NewCodeError(errorx.CodeMemoryMatchPrizeExhausted, "奖档售罄")
		}
		if entry.Remaining > 0 {
			if err := db.Model(&gamemodel.MemoryMatchEntries{}).
				Where("id = ? AND remaining > 0", entry.Id).
				Update("remaining", gorm.Expr("remaining - 1")).Error; err != nil {
				return nil, 0, err
			}
		}
		reward.HasPrize = true
		reward.PrizeKind = entry.PrizeKind
		reward.CashFen = entry.CashFen
		reward.PointsAmount = entry.PointsAmount
		reward.CouponTemplate = entry.CouponTemplateNo
		reward.EntryName = entry.Name
		reward.EntryID = entry.Id
		reward.Remaining = entry.Remaining
	} else if matched != nil {
		reward.EntryName = matched.Name
		reward.EntryID = matched.Id
	}

	// ④ settlement + 会话 COMPLETED
	rewardJSON, _ := json.Marshal(reward)
	stl := &gamemodelnc.MemoryMatchSettlements{
		SessionId:  sess.Id,
		ResultHash: finalHash,
		RewardJson: string(rewardJSON),
		Status:     mmSettlementPending,
		Version:    1,
	}
	if err := db.Create(stl).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", sess.Id).
		Updates(map[string]interface{}{
			"final_result_hash": finalHash,
			"status":            mmSessionCompleted,
			"version":           gorm.Expr("version + 1"),
			"updated_at":        time.Now(),
		}).Error; err != nil {
		return nil, 0, err
	}

	// ⑤ 擂主换擂
	if err := mmDisplaceChampion(db, sess, rules, total); err != nil {
		return nil, 0, err
	}
	return stl, total, nil
}

// mmDisplaceChampion 擂主判定（结算事务内）：total≥min 且（无擂主或严格大于现任）→
// 插事件（uk session_id 防重）+ 现任 pending 全额 → PENDING 打款流水 + 更新现任；同用户只更新。
func mmDisplaceChampion(db *gorm.DB, sess *gamemodelnc.MemoryMatchSessions, rules *mmRules, total int64) error {
	if rules.ChampionMinTotalPairs <= 0 || total < rules.ChampionMinTotalPairs {
		return nil
	}
	var ch gamemodelnc.MemoryMatchChampion
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("config_id = ?", sess.ConfigId).First(&ch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 首位擂主：插入（并发竞争撞键 → 重读再判定）
		row := &gamemodelnc.MemoryMatchChampion{
			ConfigId: sess.ConfigId, UserId: sess.UserId,
			TotalPairCount: total, PendingIncomeMinor: 0, Version: 1,
		}
		if ierr := db.Create(row).Error; ierr != nil {
			if !errors.Is(ierr, gorm.ErrDuplicatedKey) {
				return ierr
			}
			if rerr := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", sess.ConfigId).First(&ch).Error; rerr != nil {
				return rerr
			}
		} else {
			ch = *row
		}
	} else if err != nil {
		return err
	}

	if ch.UserId == sess.UserId {
		// 同用户：只刷新最高纪录（pending 保留继续累积）
		if total > ch.TotalPairCount {
			if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
				Updates(map[string]interface{}{"total_pair_count": total, "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		}
		return nil
	}
	if total <= ch.TotalPairCount {
		return nil
	}
	// 换擂：事件（防重）+ 现任 pending → PENDING 打款流水 + 更新现任
	if err := db.Create(&gamemodelnc.MemoryMatchChampionEvents{
		SessionId: sess.Id, UserId: sess.UserId, TotalPairCount: total,
	}).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return err
	}
	if ch.PendingIncomeMinor > 0 {
		if err := db.Create(&gamemodelnc.MemoryMatchChampionPayouts{
			ConfigId: sess.ConfigId, UserId: ch.UserId,
			AmountMinor: ch.PendingIncomeMinor,
			GrantNo:     fmt.Sprintf("mm-champion:%d", sess.Id),
			Status:      mmPayoutPending,
		}).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
		Updates(map[string]interface{}{
			"user_id": sess.UserId, "total_pair_count": total,
			"pending_income_minor": 0, "version": gorm.Expr("version + 1"),
		}).Error; err != nil {
		return err
	}
	return nil
}
