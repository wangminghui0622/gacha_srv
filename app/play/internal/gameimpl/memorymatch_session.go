package gameimpl

import (
	"context"
	"encoding/json"
	"errors"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewGetMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchAppearanceLogic {
	return &GetMemoryMatchAppearanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMemoryMatchAppearance 外观视图：当前选择（已存或默认）+ 已拥有正面预设
func (l *GetMemoryMatchAppearanceLogic) GetMemoryMatchAppearance(in *play.GetMemoryMatchAppearanceReq) (*play.GetMemoryMatchAppearanceResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "configId 非法")
	}
	cfg, _, err := mmLoadConfig(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, err
	}
	view, err := mmAppearanceView(l.svcCtx.DB, cfg, in.UserId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	return &play.GetMemoryMatchAppearanceResp{Appearance: view}, nil
}

func NewGetMemoryMatchConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchConfigLogic {
	return &GetMemoryMatchConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMemoryMatchConfig 配置视图：玩法规则 + 奖档 + 现任擂主 + 本人外观（选择/默认 + 已拥有正面预设）
func (l *GetMemoryMatchConfigLogic) GetMemoryMatchConfig(in *play.GetMemoryMatchConfigReq) (*play.GetMemoryMatchConfigResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "configId 非法")
	}
	cfg, rules, err := mmLoadConfig(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, err
	}
	entries, err := mmEntryOfConfig(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	item, err := mmConfigItem(cfg, rules, entries)
	if err != nil {
		return nil, err
	}
	champion, err := mmChampionView(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	appearance, err := mmAppearanceView(l.svcCtx.DB, cfg, in.UserId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	return &play.GetMemoryMatchConfigResp{
		Config:     item,
		Champion:   champion,
		Appearance: appearance,
	}, nil
}

func NewGetMemoryMatchSessionByRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchSessionByRequestLogic {
	return &GetMemoryMatchSessionByRequestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMemoryMatchSessionByRequest 按 requestId（= DTM gid）查会话：网关购买端点轮询/幂等重放用。
// paidMinor 取报价 payable（券后实付；报价缺失回退会话购买价）。不存在 → 404（网关继续轮询）。
func (l *GetMemoryMatchSessionByRequestLogic) GetMemoryMatchSessionByRequest(in *play.GetMemoryMatchSessionByRequestReq) (*play.GetMemoryMatchSessionByRequestResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var sess gamemodelnc.MemoryMatchSessions
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
		First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {

			return &play.GetMemoryMatchSessionByRequestResp{}, nil
		}
		l.Errorf("GetMemoryMatchSessionByRequest failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	paidMinor := sess.PurchasePriceMinor

	if q, qerr := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: "q_" + in.RequestId}); qerr == nil {
		paidMinor = q.GetPayableFen()
	}
	appearance, err := mmParseAppearance(sess.AppearanceJson)
	if err != nil {
		l.Errorf("GetMemoryMatchSessionByRequest parse appearance failed, sessionId: %d, err: %v", sess.Id, err)
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话外观快照异常")
	}
	return &play.GetMemoryMatchSessionByRequestResp{
		SessionId:       sess.Id,
		State:           play.MemoryMatchSessionState(sess.Status),
		PurchasedRounds: sess.PurchasedRounds,
		PaidMinor:       paidMinor,
		FacePresetId:    appearance.FacePresetID,
		BackPresetId:    appearance.BackPresetID,
	}, nil
}

func NewGetMemoryMatchSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchSessionLogic {
	return &GetMemoryMatchSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMemoryMatchSession 会话进度视图：状态 + 回合简报 + 结算（COMPLETED 后有值）+ 外观快照
func (l *GetMemoryMatchSessionLogic) GetMemoryMatchSession(in *play.GetMemoryMatchSessionReq) (*play.GetMemoryMatchSessionResp, error) {
	if in.SessionId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "sessionId 非法")
	}
	sess, err := mmLoadSession(l.svcCtx.DB, in.SessionId, in.UserId)
	if err != nil {
		return nil, err
	}
	rules, err := mmSessionRules(sess)
	if err != nil {
		return nil, err
	}

	resp := &play.GetMemoryMatchSessionResp{
		SessionId:              sess.Id,
		ConfigId:               sess.ConfigId,
		State:                  play.MemoryMatchSessionState(sess.Status),
		PurchasedRounds:        rules.RoundsPerPurchase,
		CompletedRounds:        sess.CompletedRounds,
		CurrentRound:           sess.CurrentRound,
		DestinyPatternId:       int32(sess.DestinyPatternId),
		SecondDestinyEnabled:   sess.SecondDestinyEnabled == 1,
		SecondDestinyPatternId: int32(sess.SecondDestinyPatternId),
	}
	appearance, err := mmParseAppearance(sess.AppearanceJson)
	if err == nil {
		resp.FacePresetId = appearance.FacePresetID
		resp.BackPresetId = appearance.BackPresetID
	}

	var rounds []gamemodelnc.MemoryMatchRounds
	if err := l.svcCtx.DB.Where("session_id = ?", sess.Id).Order("round_no ASC").Find(&rounds).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	for i := range rounds {
		resp.Rounds = append(resp.Rounds, &play.MemoryMatchRoundBrief{
			RoundNo:   rounds[i].RoundNo,
			State:     play.MemoryMatchRoundState(rounds[i].State),
			PairCount: rounds[i].PairCount,
		})
	}
	var stl gamemodelnc.MemoryMatchSettlements
	if err := l.svcCtx.DB.Where("session_id = ?", sess.Id).First(&stl).Error; err == nil {
		view, verr := mmSettlementView(&stl)
		if verr != nil {
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算数据异常")
		}
		resp.Settlement = view
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	return resp, nil
}

func mmParseAppearance(raw string) (*mmAppearance, error) {
	var a mmAppearance
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, err
	}
	return &a, nil
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

func NewSelectMemoryMatchDestinyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SelectMemoryMatchDestinyLogic {
	return &SelectMemoryMatchDestinyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SelectMemoryMatchDestiny 选命运（仅 AWAITING_DESTINY；同值幂等重放、异值 3039）：
// 落库命运 + 一次性创建 1..purchased 回合开局 →（第二命运启用 → AWAITING_SECOND_DESTINY，否则 PLAYING）。
func (l *SelectMemoryMatchDestinyLogic) SelectMemoryMatchDestiny(in *play.SelectMemoryMatchDestinyReq) (*play.SelectMemoryMatchDestinyResp, error) {
	if in.SessionId <= 0 || in.UserId <= 0 || in.PatternId < 1 || in.PatternId > 9 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	if _, err := mmLoadSession(l.svcCtx.DB, in.SessionId, in.UserId); err != nil {
		return nil, err
	}

	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var sess gamemodelnc.MemoryMatchSessions
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", in.SessionId, in.UserId).First(&sess).Error; err != nil {
			return errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		if sess.DestinyPatternId != 0 {
			if sess.DestinyPatternId != int64(in.PatternId) {
				return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "命运已选，异值冲突")
			}
			return nil
		}
		if sess.Status != mmSessionAwaitingDestiny {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话状态不满足选命运")
		}
		rules, err := mmSessionRules(&sess)
		if err != nil {
			return err
		}
		next := mmSessionPlaying
		if rules.SecondDestinyEnabled {
			next = mmSessionAwaitingSecond
		}
		if err := tx.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", sess.Id).
			Updates(map[string]interface{}{
				"destiny_pattern_id": in.PatternId,
				"status":             next,
				"version":            gorm.Expr("version + 1"),
				"updated_at":         time.Now(),
			}).Error; err != nil {
			return err
		}
		sess.DestinyPatternId = int64(in.PatternId)
		return mmCreateRounds(tx, &sess, rules)
	})
	if err != nil {
		return nil, mmCodeOrDBError(err)
	}
	return &play.SelectMemoryMatchDestinyResp{PatternId: in.PatternId, Status: "SELECTED"}, nil
}

func NewSelectMemoryMatchSecondDestinyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SelectMemoryMatchSecondDestinyLogic {
	return &SelectMemoryMatchSecondDestinyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SelectMemoryMatchSecondDestiny 选第二命运（仅 AWAITING_SECOND_DESTINY；同值幂等、异值 3039）→ PLAYING。
// 未启用 → 3043。
func (l *SelectMemoryMatchSecondDestinyLogic) SelectMemoryMatchSecondDestiny(in *play.SelectMemoryMatchSecondDestinyReq) (*play.SelectMemoryMatchSecondDestinyResp, error) {
	if in.SessionId <= 0 || in.UserId <= 0 || in.PatternId < 1 || in.PatternId > 9 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	if _, err := mmLoadSession(l.svcCtx.DB, in.SessionId, in.UserId); err != nil {
		return nil, err
	}

	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var sess gamemodelnc.MemoryMatchSessions
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", in.SessionId, in.UserId).First(&sess).Error; err != nil {
			return errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		if sess.SecondDestinyEnabled != 1 {
			return errorx.NewCodeError(errorx.CodeMemoryMatchSecondDestinyOff, "第二命运未启用")
		}
		if sess.SecondDestinyPatternId != 0 {
			if sess.SecondDestinyPatternId != int64(in.PatternId) {
				return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "第二命运已选，异值冲突")
			}
			return nil
		}
		if sess.Status != mmSessionAwaitingSecond {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话状态不满足选第二命运")
		}
		return tx.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", sess.Id).
			Updates(map[string]interface{}{
				"second_destiny_pattern_id": in.PatternId,
				"status":                    mmSessionPlaying,
				"version":                   gorm.Expr("version + 1"),
				"updated_at":                time.Now(),
			}).Error
	})
	if err != nil {
		return nil, mmCodeOrDBError(err)
	}
	return &play.SelectMemoryMatchSecondDestinyResp{PatternId: in.PatternId, Status: "SELECTED"}, nil
}

func NewSetMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetMemoryMatchAppearanceLogic {
	return &SetMemoryMatchAppearanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SetMemoryMatchAppearance 存外观选择（UPSERT）：
// face：0=默认；非默认须存在于配置且（默认或已拥有权益）否则 3044；
// back：0=默认；非默认须存在于配置（背面预设不校验权益，同老项目 face 权益语义）。
func (l *SetMemoryMatchAppearanceLogic) SetMemoryMatchAppearance(in *play.SetMemoryMatchAppearanceReq) (*play.SetMemoryMatchAppearanceResp, error) {
	if in.ConfigId <= 0 || in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	cfg, _, err := mmLoadConfig(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, err
	}
	faces, err := mmFacePresets(cfg)
	if err != nil {
		return nil, err
	}
	backs, err := mmBackPresets(cfg)
	if err != nil {
		return nil, err
	}

	faceID := in.FacePresetId
	if faceID == 0 {
		faceID = cfg.DefaultFacePresetId
	}
	if !memorymatch.PresetExists(faces, int(faceID)) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "正面预设不存在")
	}
	if faceID != cfg.DefaultFacePresetId {
		var ent gamemodelnc.MemoryMatchAppearanceEntitlements
		err := l.svcCtx.DB.Where("config_id = ? AND user_id = ? AND preset_id = ?",
			in.ConfigId, in.UserId, faceID).First(&ent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchAppearanceNotOwned, "外观未拥有")
		}
		if err != nil {
			return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
		}
	}
	backID := in.BackPresetId
	if backID == 0 {
		backID = cfg.DefaultBackPresetId
	}
	if !memorymatch.BackPresetExists(backs, int(backID)) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "背面预设不存在")
	}

	row := &gamemodelnc.MemoryMatchAppearanceSelections{
		ConfigId: in.ConfigId, UserId: in.UserId,
		FacePresetId: faceID, BackPresetId: backID,
		Version: 1, UpdatedAt: time.Now(),
	}
	err = l.svcCtx.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "config_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"face_preset_id": faceID,
			"back_preset_id": backID,
			"updated_at":     time.Now(),
		}),
	}).Create(row).Error
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	view, err := mmAppearanceView(l.svcCtx.DB, cfg, in.UserId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	return &play.SetMemoryMatchAppearanceResp{Appearance: view}, nil
}
