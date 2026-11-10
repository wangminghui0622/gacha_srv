package logic

import (
	"context"
	"time"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
