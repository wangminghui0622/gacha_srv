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
