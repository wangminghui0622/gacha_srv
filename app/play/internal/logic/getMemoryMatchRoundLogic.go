package logic

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

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
