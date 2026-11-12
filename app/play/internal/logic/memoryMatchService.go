package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type ApplyMemoryMatchDestinyDrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

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

type ApplyMemoryMatchPairLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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

type CompleteMemoryMatchRoundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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
	// 幂等重放快路径先于业务解析：已终局 → 补发 PENDING 后返回现有结算
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
			return nil // 并发窗口内已终局：走快路径重放
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
			// 该回合已完成：幂等返回已有视图
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
			// 末回合：终局结算（同事务）+ 事务外发奖
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

func strPtr(s string) *string { return &s }

type GetMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

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

type GetMemoryMatchConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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

type GetMemoryMatchRoundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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

type GetMemoryMatchSessionByRequestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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
			return nil, errorx.NewCodeError(errorx.CodeNotFound, "会话不存在")
		}
		l.Errorf("GetMemoryMatchSessionByRequest failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	paidMinor := sess.PurchasePriceMinor
	// 报价 payable（券后实付）经营销 RPC 读（quote_token="q_"+requestId 确定性派生；缺失回退会话购买价）
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

type GetMemoryMatchSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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

type GrantMemoryMatchPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantMemoryMatchPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantMemoryMatchPrizesLogic {
	return &GrantMemoryMatchPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantMemoryMatchPrizes SAGA 06 正向：空操作。
// 记忆翻牌购买时不发奖——奖品在终局结算事务内同步发放（grant_no="mmr:<sessionID>" 幂等补发，
// 见 mmRetryPendingSettlementGrant），06 分支仅为 SAGA 骨架占位。永不 Aborted。
func (l *GrantMemoryMatchPrizesLogic) GrantMemoryMatchPrizes(in *play.GrantMemoryMatchPrizesReq) (*play.GrantMemoryMatchPrizesResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}
	l.Infof("GrantMemoryMatchPrizes no-op, requestId: %s, sessionId: %d", in.RequestId, in.SessionId)
	return &play.GrantMemoryMatchPrizesResp{Status: "NO_PRIZES_AT_PURCHASE"}, nil
}

type GrantMemoryMatchPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantMemoryMatchPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantMemoryMatchPrizesRevertLogic {
	return &GrantMemoryMatchPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantMemoryMatchPrizesRevert SAGA 06 补偿：空操作（正向即空操作，无奖可撤）。永不 Aborted。
func (l *GrantMemoryMatchPrizesRevertLogic) GrantMemoryMatchPrizesRevert(in *play.GrantMemoryMatchPrizesReq) (*play.GrantMemoryMatchPrizesResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}
	l.Infof("GrantMemoryMatchPrizesRevert no-op, requestId: %s, sessionId: %d", in.RequestId, in.SessionId)
	return &play.GrantMemoryMatchPrizesResp{Status: "NO_PRIZES_AT_PURCHASE"}, nil
}

type LockMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockMemoryMatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockMemoryMatchLogic {
	return &LockMemoryMatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockMemoryMatch SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen/boxId 取报价行）；
//	② 配置加载 + 规则快照校验（3020）；
//	③ 外观预设校验（非默认 face 需权益 → 3044）；
//	④ 价格一致（quote.gross == 配置购买价 == 请求总额）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ session 插入 status=PENDING（uk request_id 幂等）。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockMemoryMatchLogic) LockMemoryMatch(in *play.LockMemoryMatchReq) (*play.LockMemoryMatchResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outSessionID int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockMemoryMatch", func(db *gorm.DB) error {

		// ① 报价只读校验（营销 GetQuote sub-call；quote 不存在 → Aborted）
		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID
		if in.UserId != userId {
			return status.Error(codes.Aborted, "code=400 msg=请求用户与报价不一致")
		}

		// ② 配置 + 规则快照（纯种子配置只读）
		var cfg gamemodel.MemoryMatchConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=玩法不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=玩法不可用", errorx.CodeGameConfigInvalid))
		}
		rules, err := mmConfigToRules(&cfg)
		if err != nil {
			return mmAborted(err)
		}

		// ③ 外观预设校验（0=默认；非默认 face 需权益 → 3044）
		faceID := in.FacePresetId
		if faceID == 0 {
			faceID = cfg.DefaultFacePresetId
		}
		backID := in.BackPresetId
		if backID == 0 {
			backID = cfg.DefaultBackPresetId
		}
		faces, err := mmFacePresets(&cfg)
		if err != nil {
			return mmAborted(err)
		}
		if !memorymatch.PresetExists(faces, int(faceID)) {
			return status.Error(codes.Aborted, "code=400 msg=正面预设不存在")
		}
		backs, err := mmBackPresets(&cfg)
		if err != nil {
			return mmAborted(err)
		}
		if !memorymatch.BackPresetExists(backs, int(backID)) {
			return status.Error(codes.Aborted, "code=400 msg=背面预设不存在")
		}
		if faceID != cfg.DefaultFacePresetId {
			var ent gamemodelnc.MemoryMatchAppearanceEntitlements
			err := db.Where("config_id = ? AND user_id = ? AND preset_id = ?",
				in.ConfigId, userId, faceID).First(&ent).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=外观未拥有", errorx.CodeMemoryMatchAppearanceNotOwned))
			}
			if err != nil {
				return err
			}
		}

		// ④ 价格一致：quote.gross == 配置购买价 == 请求总额；quote.BoxId 绑定配置（M10 报价预留字段）
		if quote.GrossFen != cfg.PurchasePriceMinor || in.TotalPriceFen != cfg.PurchasePriceMinor {
			return status.Error(codes.Aborted, "code=400 msg=购买价格校验不一致")
		}
		if quote.BoxID != 0 && quote.BoxID != in.ConfigId {
			return status.Error(codes.Aborted, "code=400 msg=报价与玩法不匹配")
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，读回）
		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockMemoryMatch idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			outSessionID = 0
			return nil
		}

		// ⑥ session 插入 PENDING（uk request_id 幂等：撞键 = 重试重放 → 读回）
		appearance := mmAppearance{FacePresetID: faceID, BackPresetID: backID}
		appearanceJSON, _ := json.Marshal(appearance)
		secondEnabled := int64(0)
		if rules.SecondDestinyEnabled {
			secondEnabled = 1
		}
		row := &gamemodelnc.MemoryMatchSessions{
			UserId:               userId,
			ConfigId:             in.ConfigId,
			RequestId:            in.RequestId,
			EngineVersion:        1,
			PurchasedRounds:      rules.RoundsPerPurchase,
			PurchasePriceMinor:   rules.PurchasePriceMinor,
			RulesJson:            mmRulesJSON(rules),
			AppearanceJson:       string(appearanceJSON),
			DestinyPatternId:     0,
			SecondDestinyEnabled: secondEnabled,
			CompletedRounds:      0,
			CurrentRound:         1,
			SkipRequested:        0,
			Status:               mmSessionPending,
			Version:              1,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}
		if err := db.Create(row).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				// 重放：读回已有会话
				var exist gamemodelnc.MemoryMatchSessions
				if rerr := db.Where("request_id = ?", in.RequestId).First(&exist).Error; rerr != nil {
					return rerr
				}
				outSessionID = exist.Id
				return nil
			}
			return err
		}
		outSessionID = row.Id
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockMemoryMatch aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockMemoryMatch failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan || outSessionID == 0 {
		// 屏障级跳过或幂等读回：读库如实返回
		return l.readBackLock(in.RequestId)
	}
	l.Infof("LockMemoryMatch success, requestId: %s, sessionId: %d", in.RequestId, outSessionID)
	return &play.LockMemoryMatchResp{SessionId: outSessionID, Status: "LOCKED"}, nil
}

// readBackLock 屏障跳过/幂等重放时读会话如实返回
func (l *LockMemoryMatchLogic) readBackLock(requestId string) (*play.LockMemoryMatchResp, error) {
	var sess gamemodelnc.MemoryMatchSessions
	if err := l.svcCtx.DB.Where("request_id = ?", requestId).First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.Internal, "会话不存在")
		}
		l.Errorf("readBackLock failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackLock: requestId: %s, sessionId: %d, status: %d", requestId, sess.Id, sess.Status)
	return &play.LockMemoryMatchResp{SessionId: sess.Id, Status: "LOCKED"}, nil
}

type LockMemoryMatchRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockMemoryMatchRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockMemoryMatchRevertLogic {
	return &LockMemoryMatchRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockMemoryMatchRevert SAGA 03 补偿：会话 PENDING/AWAITING_DESTINY → EXPIRED（盲撤销幂等）；
// 已开始游玩（AWAITING_SECOND/PLAYING/COMPLETED/EXPIRED）→ 跳过不打扰。报价释放由 payment 04 补偿负责。
func (l *LockMemoryMatchRevertLogic) LockMemoryMatchRevert(in *play.LockMemoryMatchReq) (*play.LockMemoryMatchResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outSessionID int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "lockMemoryMatchRevert", func(db *gorm.DB) error {

		var sess gamemodelnc.MemoryMatchSessions
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 03 正向从未成功：无会话可撤销
		}
		if err != nil {
			return err
		}
		outSessionID = sess.Id
		if sess.Status == mmSessionPending || sess.Status == mmSessionAwaitingDestiny {
			if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).
				Where("id = ? AND status = ?", sess.Id, sess.Status).
				Updates(map[string]interface{}{
					"status":     mmSessionExpired,
					"version":    gorm.Expr("version + 1"),
					"updated_at": time.Now(),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockMemoryMatchRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("LockMemoryMatchRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan || outSessionID == 0 {
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outSessionID = sess.Id
		}
	}
	l.Infof("LockMemoryMatchRevert success, requestId: %s, sessionId: %d", in.RequestId, outSessionID)
	return &play.LockMemoryMatchResp{SessionId: outSessionID, Status: "RELEASED"}, nil
}

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

// mmGrantOne 单奖幂等发放（grant_no 全 uk 幂等；统一走 playkit.GrantOne）
func mmGrantOne(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, grantNo string, prizeKind, cashFen, points int64, couponTemplateNo string) error {
	return playkit.GrantOne(ctx, svcCtx, userID, grantNo, playkit.Prize{
		Kind:             prizeKind,
		CashFen:          cashFen,
		Points:           points,
		CouponTemplateNo: couponTemplateNo,
	})
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

type SelectMemoryMatchDestinyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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
			return nil // 同值幂等重放
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

type SelectMemoryMatchSecondDestinyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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
			return nil // 同值幂等重放
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

type SetMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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

	// UPSERT（uk(config_id,user_id)）
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

type SettleMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleMemoryMatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleMemoryMatchLogic {
	return &SettleMemoryMatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleMemoryMatch SAGA 05 正向：barrier 事务内
//
//	① 会话 FOR UPDATE（uk request_id 幂等重放：非 PENDING → 读回当前状态成功返回）；
//	② PENDING → AWAITING_DESTINY：engine_seed crypto/rand 落库；
//	③ 擂主分成：现任擂主 pending += 购买价×bps/10000（无擂主/未启用 → 跳过；
//	   记账到会话 champion_credit_user_id/champion_share_minor 供 Revert 等额回扣）。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleMemoryMatchLogic) SettleMemoryMatch(in *play.SettleMemoryMatchReq) (*play.SettleMemoryMatchResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outStatus int64
	var outSessionID int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleMemoryMatch", func(db *gorm.DB) error {

		// ① 会话 FOR UPDATE（request_id 唯一键幂等）
		var sess gamemodelnc.MemoryMatchSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=会话不存在", errorx.CodeMemoryMatchConflict))
			}
			return err
		}
		if in.SessionId > 0 && sess.Id != in.SessionId || sess.UserId != in.UserId || sess.ConfigId != in.ConfigId {
			return status.Error(codes.Aborted, "code=400 msg=会话参数不一致")
		}
		outStatus = sess.Status
		outSessionID = sess.Id
		if sess.Status != mmSessionPending {
			// 幂等重放（已结算/已游玩）：读回当前状态成功返回
			l.Infof("SettleMemoryMatch idempotent: session %d already at status %d", sess.Id, sess.Status)
			return nil
		}

		rules, err := mmSessionRules(&sess)
		if err != nil {
			return mmAborted(err)
		}

		// ② PENDING → AWAITING_DESTINY + 引擎种子
		if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ? AND status = ?", sess.Id, mmSessionPending).
			Updates(map[string]interface{}{
				"engine_seed": mmEngineSeedHex(),
				"status":      mmSessionAwaitingDestiny,
				"version":     gorm.Expr("version + 1"),
				"updated_at":  time.Now(),
			}).Error; err != nil {
			return err
		}

		// ③ 擂主分成（现任擂主存在且玩法启用擂主时；记账供 Revert 回扣）
		if rules.ChampionMinTotalPairs > 0 && rules.ChampionShareBps > 0 {
			share := memorymatch.ChampionCreditMinor(sess.PurchasePriceMinor, rules.ChampionShareBps)
			if share > 0 {
				var ch gamemodelnc.MemoryMatchChampion
				err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("config_id = ?", sess.ConfigId).First(&ch).Error
				if err == nil {
					if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
						Update("pending_income_minor", gorm.Expr("pending_income_minor + ?", share)).Error; err != nil {
						return err
					}
					if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", sess.Id).
						Updates(map[string]interface{}{
							"champion_credit_user_id": ch.UserId,
							"champion_share_minor":    share,
						}).Error; err != nil {
						return err
					}
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				// 无擂主行：无现任可分成，跳过
			}
		}
		outStatus = mmSessionAwaitingDestiny
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleMemoryMatch aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleMemoryMatch failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功）：读库如实返回
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outStatus = sess.Status
			outSessionID = sess.Id
		}
	}
	l.Infof("SettleMemoryMatch success, requestId: %s, sessionId: %d, status: %d", in.RequestId, outSessionID, outStatus)
	return &play.SettleMemoryMatchResp{
		SessionId: outSessionID,
		State:     play.MemoryMatchSessionState(outStatus),
	}, nil
}

type SettleMemoryMatchRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleMemoryMatchRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleMemoryMatchRevertLogic {
	return &SettleMemoryMatchRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleMemoryMatchRevert SAGA 05 补偿：会话 AWAITING_DESTINY → PENDING（清引擎种子、回扣擂主分成）。
// 已进入游玩（AWAITING_SECOND/PLAYING/COMPLETED/EXPIRED）→ 跳过不打扰（盲撤销幂等）。
//
// 擂主分成回扣竞态守卫：仅当现任擂主仍是结算时记账的 champion_credit_user_id 才扣回
// （max(0, pending-share) 兜底）；擂主已易主则接受泄漏，绝不从新擂主处偷扣。
func (l *SettleMemoryMatchRevertLogic) SettleMemoryMatchRevert(in *play.SettleMemoryMatchReq) (*play.SettleMemoryMatchResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outStatus int64
	var outSessionID int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleMemoryMatchRevert", func(db *gorm.DB) error {

		var sess gamemodelnc.MemoryMatchSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 05 正向从未成功：无可回滚
			}
			return err
		}
		outSessionID = sess.Id
		outStatus = sess.Status
		if sess.Status != mmSessionAwaitingDestiny {
			return nil // 非待命运状态：跳过不打扰
		}

		// ① 回扣擂主分成（竞态守卫：现任擂主仍是记账用户才扣，max(0,...) 兜底）
		if sess.ChampionCreditUserId > 0 && sess.ChampionShareMinor > 0 {
			var ch gamemodelnc.MemoryMatchChampion
			if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", sess.ConfigId).First(&ch).Error; err == nil {
				if ch.UserId == sess.ChampionCreditUserId {
					if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
						Update("pending_income_minor",
							gorm.Expr("GREATEST(0, pending_income_minor - ?)", sess.ChampionShareMinor)).Error; err != nil {
						return err
					}
				}
				// 擂主已易主：接受泄漏，不偷扣新擂主
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		// ② AWAITING_DESTINY → PENDING + 清种子 + 清记账标记（盲更新幂等）
		if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).
			Where("id = ? AND status = ?", sess.Id, mmSessionAwaitingDestiny).
			Updates(map[string]interface{}{
				"engine_seed":             gorm.Expr("NULL"),
				"champion_credit_user_id": 0,
				"champion_share_minor":    0,
				"status":                  mmSessionPending,
				"version":                 gorm.Expr("version + 1"),
				"updated_at":              time.Now(),
			}).Error; err != nil {
			return err
		}
		outStatus = mmSessionPending
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleMemoryMatchRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleMemoryMatchRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/已补偿）：读库如实返回
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outStatus = sess.Status
			outSessionID = sess.Id
		}
	}
	l.Infof("SettleMemoryMatchRevert success, requestId: %s, sessionId: %d, status: %d", in.RequestId, outSessionID, outStatus)
	return &play.SettleMemoryMatchResp{
		SessionId: outSessionID,
		State:     play.MemoryMatchSessionState(outStatus),
	}, nil
}

type SkipMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
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
	// 幂等重放快路径先于业务解析：已终局 → 补发 PENDING 后返回现有结算
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
			return nil // 并发窗口内已终局：走快路径重放
		}
		if locked.Status != mmSessionPlaying {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "会话状态冲突")
		}
		engineSeed, err := mmEngineSeedBytes(*locked.EngineSeed)
		if err != nil {
			return errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "引擎种子异常")
		}
		// 当前回合起 AutoResolve 至 purchased（已 RESOLVED 跳过 = 幂等续跑）
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
