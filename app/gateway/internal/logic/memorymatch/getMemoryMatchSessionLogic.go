package memorymatch

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchSessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMemoryMatchSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchSessionLogic {
	return &GetMemoryMatchSessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetMemoryMatchSession 会话进度：状态/回合简报/终局结算/外观快照（GET /memory-match/sessions/:sessionId）
func (l *GetMemoryMatchSessionLogic) GetMemoryMatchSession(sessionId int64) (resp *types.MemoryMatchSessionResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetMemoryMatchSession(l.ctx, &playClient.GetMemoryMatchSessionReq{
		SessionId: sessionId,
		UserId:    userId,
	})
	if err != nil {
		l.Errorf("GetMemoryMatchSession failed, sessionId: %d, err: %v", sessionId, err)
		return nil, errorx.FromError(err)
	}
	rounds := make([]types.MemoryMatchRoundBrief, 0, len(r.Rounds))
	for _, b := range r.Rounds {
		rounds = append(rounds, types.MemoryMatchRoundBrief{
			RoundNo:   b.RoundNo,
			State:     int64(b.State),
			PairCount: b.PairCount,
		})
	}
	return &types.MemoryMatchSessionResp{
		SessionId:              r.SessionId,
		ConfigId:               r.ConfigId,
		State:                  int64(r.State),
		PurchasedRounds:        r.PurchasedRounds,
		CompletedRounds:        r.CompletedRounds,
		CurrentRound:           r.CurrentRound,
		DestinyPatternId:       r.DestinyPatternId,
		SecondDestinyEnabled:   r.SecondDestinyEnabled,
		SecondDestinyPatternId: r.SecondDestinyPatternId,
		Rounds:                 rounds,
		Settlement:             settlementToView(r.Settlement),
		FacePresetId:           r.FacePresetId,
		BackPresetId:           r.BackPresetId,
	}, nil
}
