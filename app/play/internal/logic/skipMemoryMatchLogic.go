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
