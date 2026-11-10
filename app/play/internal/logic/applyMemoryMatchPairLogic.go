package logic

import (
	"context"
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
