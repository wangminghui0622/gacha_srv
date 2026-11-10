package logic

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
