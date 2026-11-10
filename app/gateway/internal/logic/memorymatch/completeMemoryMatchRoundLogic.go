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

type CompleteMemoryMatchRoundLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCompleteMemoryMatchRoundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteMemoryMatchRoundLogic {
	return &CompleteMemoryMatchRoundLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CompleteMemoryMatchRound 完成回合（POST .../complete；未终局 6023；末回合触发终局结算+发奖+擂主）
func (l *CompleteMemoryMatchRoundLogic) CompleteMemoryMatchRound(sessionId, roundNo int64) (resp *types.MemoryMatchCompleteResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.CompleteMemoryMatchRound(l.ctx, &playClient.CompleteMemoryMatchRoundReq{
		SessionId: sessionId,
		UserId:    userId,
		RoundNo:   roundNo,
	})
	if err != nil {
		l.Errorf("CompleteMemoryMatchRound failed, sessionId: %d, roundNo: %d, err: %v", sessionId, roundNo, err)
		return nil, errorx.FromError(err)
	}
	return &types.MemoryMatchCompleteResp{
		Round:        roundToView(r.Round),
		SessionState: int64(r.SessionState),
		Settlement:   settlementToView(r.Settlement),
	}, nil
}
