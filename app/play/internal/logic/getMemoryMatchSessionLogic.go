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
