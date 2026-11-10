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

type GetMemoryMatchRoundLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMemoryMatchRoundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchRoundLogic {
	return &GetMemoryMatchRoundLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetMemoryMatchRound 回合视图（GET /memory-match/sessions/:sessionId/rounds/:roundNo；现场派生校验）
func (l *GetMemoryMatchRoundLogic) GetMemoryMatchRound(sessionId, roundNo int64) (resp *types.MemoryMatchRoundResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetMemoryMatchRound(l.ctx, &playClient.GetMemoryMatchRoundReq{
		SessionId: sessionId,
		UserId:    userId,
		RoundNo:   roundNo,
	})
	if err != nil {
		l.Errorf("GetMemoryMatchRound failed, sessionId: %d, roundNo: %d, err: %v", sessionId, roundNo, err)
		return nil, errorx.FromError(err)
	}
	view := roundToView(r)
	return &view, nil
}
