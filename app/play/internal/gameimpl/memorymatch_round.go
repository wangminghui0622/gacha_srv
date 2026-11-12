package gameimpl

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewApplyMemoryMatchDestinyDrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyMemoryMatchDestinyDrawLogic {
	return &ApplyMemoryMatchDestinyDrawLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApplyMemoryMatchDestinyDraw 命运抽牌：消耗 1 命运资源抽 1 张（要求手牌无同图案对子、堆未空、
// 手牌未满）；抽到命运图案 → 命运资源+1。错误映射：不可用 → 3042、已终局 → 3041、状态错 → 3039。
func (l *ApplyMemoryMatchDestinyDrawLogic) ApplyMemoryMatchDestinyDraw(in *play.ApplyMemoryMatchDestinyDrawReq) (*play.ApplyMemoryMatchDestinyDrawResp, error) {
	if in.SessionId <= 0 || in.UserId <= 0 || in.RoundNo <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	if _, err := mmLoadSession(l.svcCtx.DB, in.SessionId, in.UserId); err != nil {
		return nil, err
	}

	var view *play.GetMemoryMatchRoundResp
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var sess gamemodelnc.MemoryMatchSessions
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", in.SessionId, in.UserId).First(&sess).Error; err != nil {
			return errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		if sess.Status != mmSessionPlaying || sess.CurrentRound != in.RoundNo {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话状态冲突")
		}
		rules, err := mmSessionRules(&sess)
		if err != nil {
			return err
		}
		var round gamemodelnc.MemoryMatchRounds
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_id = ? AND round_no = ?", in.SessionId, in.RoundNo).First(&round).Error; err != nil {
			return errorx.NewCodeError(errorx.CodeNotFound, "回合不存在")
		}
		if round.State != mmRoundPlaying {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态冲突")
		}
		st, err := mmParseRoundState(round.StateJson)
		if err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态异常")
		}
		engineSeed, err := mmEngineSeedBytes(*sess.EngineSeed)
		if err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "引擎种子异常")
		}
		roundSeed := memorymatch.RoundSeed(engineSeed, sess.Id, round.RoundNo)
		if err := memorymatch.ValidateOpening(st, roundSeed, int(rules.MaxPairCount)); err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合开局校验失败")
		}
		if err := st.ApplyDestinyDraw(int(rules.MaxPairCount),
			int32(sess.DestinyPatternId), int32(sess.SecondDestinyPatternId),
			rules.SecondDestinyEnabled); err != nil {
			return mmMapEngineErr(err)
		}
		if err := tx.Model(&gamemodelnc.MemoryMatchRounds{}).Where("id = ?", round.Id).
			Updates(map[string]interface{}{
				"state_json": mmRoundStateJSON(st),
				"pair_count": st.PairCount,
				"version":    gorm.Expr("version + 1"),
				"updated_at": time.Now(),
			}).Error; err != nil {
			return err
		}
		round.StateJson = mmRoundStateJSON(st)
		round.PairCount = int64(st.PairCount)
		view, err = mmRoundView(&sess, rules, &round, *sess.EngineSeed)
		return err
	})
	if err != nil {
		return nil, mmCodeOrDBError(err)
	}
	return &play.ApplyMemoryMatchDestinyDrawResp{Round: view}, nil
}

func NewApplyMemoryMatchPairLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyMemoryMatchPairLogic {
	return &ApplyMemoryMatchPairLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApplyMemoryMatchPair 配对（两卡同图案且都在手牌）：移除两张 + pair_count++（命运图案 → 命运资源+1）
// + 补牌（补入命运图案 → 命运资源+1）；错误映射：无效配对 → 400、已终局 → 3041、状态错 → 3039。
func (l *ApplyMemoryMatchPairLogic) ApplyMemoryMatchPair(in *play.ApplyMemoryMatchPairReq) (*play.ApplyMemoryMatchPairResp, error) {
	if in.SessionId <= 0 || in.UserId <= 0 || in.RoundNo <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	if in.FirstCard < 1 || in.FirstCard > 54 || in.SecondCard < 1 || in.SecondCard > 54 ||
		in.FirstCard == in.SecondCard {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "配对无效")
	}
	if _, err := mmLoadSession(l.svcCtx.DB, in.SessionId, in.UserId); err != nil {
		return nil, err
	}

	var view *play.GetMemoryMatchRoundResp
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var sess gamemodelnc.MemoryMatchSessions
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", in.SessionId, in.UserId).First(&sess).Error; err != nil {
			return errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		if sess.Status != mmSessionPlaying || sess.CurrentRound != in.RoundNo {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话状态冲突")
		}
		rules, err := mmSessionRules(&sess)
		if err != nil {
			return err
		}
		var round gamemodelnc.MemoryMatchRounds
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_id = ? AND round_no = ?", in.SessionId, in.RoundNo).First(&round).Error; err != nil {
			return errorx.NewCodeError(errorx.CodeNotFound, "回合不存在")
		}
		if round.State != mmRoundPlaying {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态冲突")
		}
		st, err := mmParseRoundState(round.StateJson)
		if err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态异常")
		}
		engineSeed, err := mmEngineSeedBytes(*sess.EngineSeed)
		if err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "引擎种子异常")
		}
		roundSeed := memorymatch.RoundSeed(engineSeed, sess.Id, round.RoundNo)
		if err := memorymatch.ValidateOpening(st, roundSeed, int(rules.MaxPairCount)); err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合开局校验失败")
		}
		if err := st.ApplyPair(int(rules.MaxPairCount),
			int32(sess.DestinyPatternId), int32(sess.SecondDestinyPatternId),
			rules.SecondDestinyEnabled, in.FirstCard, in.SecondCard); err != nil {
			return mmMapEngineErr(err)
		}
		if err := tx.Model(&gamemodelnc.MemoryMatchRounds{}).Where("id = ?", round.Id).
			Updates(map[string]interface{}{
				"state_json": mmRoundStateJSON(st),
				"pair_count": st.PairCount,
				"version":    gorm.Expr("version + 1"),
				"updated_at": time.Now(),
			}).Error; err != nil {
			return err
		}
		round.StateJson = mmRoundStateJSON(st)
		round.PairCount = int64(st.PairCount)
		view, err = mmRoundView(&sess, rules, &round, *sess.EngineSeed)
		return err
	})
	if err != nil {
		return nil, mmCodeOrDBError(err)
	}
	return &play.ApplyMemoryMatchPairResp{Round: view}, nil
}

func NewCompleteMemoryMatchRoundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteMemoryMatchRoundLogic {
	return &CompleteMemoryMatchRoundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CompleteMemoryMatchRound 完成回合（Terminal 后；未终局 → 3040）：
// 落 result_json/result_hash → completed+1；末回合触发终局结算+发奖+擂主。
// 幂等：会话已 COMPLETED 快路径重放（补发 PENDING 结算奖/擂主打款）；回合已 RESOLVED 返回已有视图。
func (l *CompleteMemoryMatchRoundLogic) CompleteMemoryMatchRound(in *play.CompleteMemoryMatchRoundReq) (*play.CompleteMemoryMatchRoundResp, error) {
	if in.SessionId <= 0 || in.UserId <= 0 || in.RoundNo <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	sess, err := mmLoadSession(l.svcCtx.DB, in.SessionId, in.UserId)
	if err != nil {
		return nil, err
	}

	if sess.Status == mmSessionCompleted {
		if err := mmRetryPendingSettlementGrant(l.ctx, l.svcCtx, sess); err != nil {
			return nil, mmCodeOrDBError(err)
		}
		if err := mmRetryChampionPayouts(l.ctx, l.svcCtx, sess.ConfigId); err != nil {
			return nil, mmCodeOrDBError(err)
		}
		return l.completedResp(sess, in.RoundNo)
	}
	rules, err := mmSessionRules(sess)
	if err != nil {
		return nil, err
	}

	var view *play.GetMemoryMatchRoundResp
	var settlement *play.MemoryMatchSettlementView
	finalSettled := false
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var locked gamemodelnc.MemoryMatchSessions
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", in.SessionId, in.UserId).First(&locked).Error; err != nil {
			return errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		if locked.Status == mmSessionCompleted {
			*sess = locked
			return nil
		}
		if locked.Status != mmSessionPlaying {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话状态冲突")
		}
		var round gamemodelnc.MemoryMatchRounds
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_id = ? AND round_no = ?", in.SessionId, in.RoundNo).First(&round).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorx.NewCodeError(errorx.CodeNotFound, "回合不存在")
			}
			return err
		}
		if round.State == mmRoundResolved {

			view, err = mmRoundView(&locked, rules, &round, *locked.EngineSeed)
			return err
		}
		if round.State != mmRoundPlaying {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态冲突")
		}
		if in.RoundNo != locked.CurrentRound {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合与当前进度不一致")
		}
		st, err := mmParseRoundState(round.StateJson)
		if err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态异常")
		}
		engineSeed, err := mmEngineSeedBytes(*locked.EngineSeed)
		if err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "引擎种子异常")
		}
		roundSeed := memorymatch.RoundSeed(engineSeed, locked.Id, round.RoundNo)
		if err := memorymatch.ValidateOpening(st, roundSeed, int(rules.MaxPairCount)); err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合开局校验失败")
		}
		if !st.IsTerminal(int(rules.MaxPairCount)) {
			return errorx.NewCodeError(errorx.CodeMemoryMatchRoundNotFinished, "回合未完成")
		}
		resultJSON, resultHash := memorymatch.RoundResult(st.PairCount)
		if err := tx.Model(&gamemodelnc.MemoryMatchRounds{}).Where("id = ?", round.Id).
			Updates(map[string]interface{}{
				"state":       mmRoundResolved,
				"result_json": string(resultJSON),
				"result_hash": hex.EncodeToString(resultHash),
				"version":     gorm.Expr("version + 1"),
				"updated_at":  time.Now(),
			}).Error; err != nil {
			return err
		}
		round.State = mmRoundResolved
		round.StateJson = mmRoundStateJSON(st)
		round.ResultJson = strPtr(string(resultJSON))
		round.ResultHash = strPtr(hex.EncodeToString(resultHash))

		completed := locked.CompletedRounds + 1
		if err := tx.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", locked.Id).
			Updates(map[string]interface{}{
				"completed_rounds": completed,
				"current_round":    locked.CurrentRound + 1,
				"version":          gorm.Expr("version + 1"),
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		locked.CompletedRounds = completed
		locked.CurrentRound++

		view, err = mmRoundView(&locked, rules, &round, *locked.EngineSeed)
		if err != nil {
			return err
		}
		if completed == rules.RoundsPerPurchase {

			stl, _, err := mmFinalSettlement(tx, &locked, rules)
			if err != nil {
				return err
			}
			locked.Status = mmSessionCompleted
			stlView, err := mmSettlementView(stl)
			if err != nil {
				return err
			}
			settlement = stlView
			finalSettled = true
		}
		*sess = locked
		return nil
	})
	if err != nil {
		return nil, mmCodeOrDBError(err)
	}
	if finalSettled {
		if err := mmRetryPendingSettlementGrant(l.ctx, l.svcCtx, sess); err != nil {
			return nil, mmCodeOrDBError(err)
		}
		if err := mmRetryChampionPayouts(l.ctx, l.svcCtx, sess.ConfigId); err != nil {
			return nil, mmCodeOrDBError(err)
		}
		// 补发把结算 status 1→2，重读最新行再回响应（避免返回过期的 PENDING 视图）
		var stl gamemodelnc.MemoryMatchSettlements
		if err := l.svcCtx.DB.Where("session_id = ?", sess.Id).First(&stl).Error; err != nil {
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算缺失")
		}
		settlement, err = mmSettlementView(&stl)
		if err != nil {
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算数据异常")
		}
		return &play.CompleteMemoryMatchRoundResp{
			Round:        view,
			SessionState: play.MemoryMatchSessionState(sess.Status),
			Settlement:   settlement,
		}, nil
	}
	return &play.CompleteMemoryMatchRoundResp{
		Round:        view,
		SessionState: play.MemoryMatchSessionState(sess.Status),
	}, nil
}

// completedResp 已终局幂等重放：返回请求回合视图 + 现有结算（发奖已补发，status 为最新）
func (l *CompleteMemoryMatchRoundLogic) completedResp(sess *gamemodelnc.MemoryMatchSessions, roundNo int64) (*play.CompleteMemoryMatchRoundResp, error) {
	var round gamemodelnc.MemoryMatchRounds
	if err := l.svcCtx.DB.Where("session_id = ? AND round_no = ?", sess.Id, roundNo).First(&round).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeNotFound, "回合不存在")
		}
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	rules, err := mmSessionRules(sess)
	if err != nil {
		return nil, err
	}
	roundView, err := mmRoundView(sess, rules, &round, *sess.EngineSeed)
	if err != nil {
		return nil, err
	}
	var stl gamemodelnc.MemoryMatchSettlements
	if err := l.svcCtx.DB.Where("session_id = ?", sess.Id).First(&stl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算缺失")
		}
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	view, err := mmSettlementView(&stl)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算数据异常")
	}
	return &play.CompleteMemoryMatchRoundResp{
		Round:        roundView,
		SessionState: play.MemoryMatchSessionState(sess.Status),
		Settlement:   view,
	}, nil
}

func NewGetMemoryMatchRoundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchRoundLogic {
	return &GetMemoryMatchRoundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMemoryMatchRound 回合视图（现场派生校验 opening + 行持久化状态）
func (l *GetMemoryMatchRoundLogic) GetMemoryMatchRound(in *play.GetMemoryMatchRoundReq) (*play.GetMemoryMatchRoundResp, error) {
	if in.SessionId <= 0 || in.RoundNo <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	sess, err := mmLoadSession(l.svcCtx.DB, in.SessionId, in.UserId)
	if err != nil {
		return nil, err
	}
	rules, err := mmSessionRules(sess)
	if err != nil {
		return nil, err
	}
	if in.RoundNo > rules.RoundsPerPurchase {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "roundNo 非法")
	}
	var round gamemodelnc.MemoryMatchRounds
	if err := l.svcCtx.DB.Where("session_id = ? AND round_no = ?", in.SessionId, in.RoundNo).
		First(&round).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeNotFound, "回合不存在")
		}
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	seed := ""
	if sess.EngineSeed != nil {
		seed = *sess.EngineSeed
	}
	return mmRoundView(sess, rules, &round, seed)
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

func NewSkipMemoryMatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SkipMemoryMatchLogic {
	return &SkipMemoryMatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SkipMemoryMatch 跳过：当前回合续玩 AutoResolve 至终局 + 剩余回合整局 AutoResolve（老项目
// ManualMoveThenSkipResumesSameRound/SkipMatchesSequentialAutoResolution 语义）→ 终局结算+发奖+擂主。
// 幂等：已 COMPLETED 快路径重放（补发 PENDING）；已跳过的回合 RESOLVED 续跑。
func (l *SkipMemoryMatchLogic) SkipMemoryMatch(in *play.SkipMemoryMatchReq) (*play.SkipMemoryMatchResp, error) {
	if in.SessionId <= 0 || in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	sess, err := mmLoadSession(l.svcCtx.DB, in.SessionId, in.UserId)
	if err != nil {
		return nil, err
	}

	if sess.Status == mmSessionCompleted {
		if err := mmRetryPendingSettlementGrant(l.ctx, l.svcCtx, sess); err != nil {
			return nil, mmCodeOrDBError(err)
		}
		if err := mmRetryChampionPayouts(l.ctx, l.svcCtx, sess.ConfigId); err != nil {
			return nil, mmCodeOrDBError(err)
		}
		var stl gamemodelnc.MemoryMatchSettlements
		if err := l.svcCtx.DB.Where("session_id = ?", sess.Id).First(&stl).Error; err != nil {
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算缺失")
		}
		view, err := mmSettlementView(&stl)
		if err != nil {
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算数据异常")
		}
		return &play.SkipMemoryMatchResp{
			SessionId:    sess.Id,
			SessionState: play.MemoryMatchSessionState(sess.Status),
			Settlement:   view,
		}, nil
	}
	rules, err := mmSessionRules(sess)
	if err != nil {
		return nil, err
	}

	var settlement *play.MemoryMatchSettlementView
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var locked gamemodelnc.MemoryMatchSessions
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", in.SessionId, in.UserId).First(&locked).Error; err != nil {
			return errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		if locked.Status == mmSessionCompleted {
			*sess = locked
			return nil
		}
		if locked.Status != mmSessionPlaying {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话状态冲突")
		}
		engineSeed, err := mmEngineSeedBytes(*locked.EngineSeed)
		if err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "引擎种子异常")
		}

		for roundNo := locked.CurrentRound; roundNo <= rules.RoundsPerPurchase; roundNo++ {
			var round gamemodelnc.MemoryMatchRounds
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("session_id = ? AND round_no = ?", locked.Id, roundNo).First(&round).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合缺失")
				}
				return err
			}
			if round.State == mmRoundResolved {
				continue
			}
			if round.State != mmRoundPlaying {
				return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态冲突")
			}
			st, err := mmParseRoundState(round.StateJson)
			if err != nil {
				return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合状态异常")
			}
			roundSeed := memorymatch.RoundSeed(engineSeed, locked.Id, round.RoundNo)
			if err := memorymatch.ValidateOpening(st, roundSeed, int(rules.MaxPairCount)); err != nil {
				return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合开局校验失败")
			}
			if err := st.AutoResolve(int(rules.MaxPairCount),
				int32(locked.DestinyPatternId), int32(locked.SecondDestinyPatternId),
				rules.SecondDestinyEnabled); err != nil {
				return mmMapEngineErr(err)
			}
			resultJSON, resultHash := memorymatch.RoundResult(st.PairCount)
			if err := tx.Model(&gamemodelnc.MemoryMatchRounds{}).Where("id = ?", round.Id).
				Updates(map[string]interface{}{
					"state":       mmRoundResolved,
					"state_json":  mmRoundStateJSON(st),
					"pair_count":  st.PairCount,
					"result_json": string(resultJSON),
					"result_hash": hex.EncodeToString(resultHash),
					"version":     gorm.Expr("version + 1"),
					"updated_at":  time.Now(),
				}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", locked.Id).
			Updates(map[string]interface{}{
				"completed_rounds": rules.RoundsPerPurchase,
				"skip_requested":   1,
				"version":          gorm.Expr("version + 1"),
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		locked.CompletedRounds = rules.RoundsPerPurchase
		locked.SkipRequested = 1
		stl, _, err := mmFinalSettlement(tx, &locked, rules)
		if err != nil {
			return err
		}
		locked.Status = mmSessionCompleted
		view, err := mmSettlementView(stl)
		if err != nil {
			return err
		}
		settlement = view
		*sess = locked
		return nil
	})
	if err != nil {
		return nil, mmCodeOrDBError(err)
	}
	if err := mmRetryPendingSettlementGrant(l.ctx, l.svcCtx, sess); err != nil {
		return nil, mmCodeOrDBError(err)
	}
	if err := mmRetryChampionPayouts(l.ctx, l.svcCtx, sess.ConfigId); err != nil {
		return nil, mmCodeOrDBError(err)
	}
	// 补发把结算 status 1→2，重读最新行再回响应（避免返回过期的 PENDING 视图）
	var stl gamemodelnc.MemoryMatchSettlements
	if err := l.svcCtx.DB.Where("session_id = ?", sess.Id).First(&stl).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算缺失")
	}
	settlement, err = mmSettlementView(&stl)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "结算数据异常")
	}
	return &play.SkipMemoryMatchResp{
		SessionId:    sess.Id,
		SessionState: play.MemoryMatchSessionState(sess.Status),
		Settlement:   settlement,
	}, nil
}
