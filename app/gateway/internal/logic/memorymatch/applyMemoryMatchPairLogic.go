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

type ApplyMemoryMatchPairLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApplyMemoryMatchPairLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyMemoryMatchPairLogic {
	return &ApplyMemoryMatchPairLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ApplyMemoryMatchPair 配对（POST .../pair；两卡同图案移除+补牌，命运图案配对 +1 可用命运）
func (l *ApplyMemoryMatchPairLogic) ApplyMemoryMatchPair(req *types.MemoryMatchPairReq) (resp *types.MemoryMatchRoundActionResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.ApplyMemoryMatchPair(l.ctx, &playClient.ApplyMemoryMatchPairReq{
		SessionId:  req.SessionId,
		UserId:     userId,
		RoundNo:    req.RoundNo,
		FirstCard:  req.FirstCard,
		SecondCard: req.SecondCard,
	})
	if err != nil {
		l.Errorf("ApplyMemoryMatchPair failed, sessionId: %d, roundNo: %d, err: %v", req.SessionId, req.RoundNo, err)
		return nil, errorx.FromError(err)
	}
	return &types.MemoryMatchRoundActionResp{Round: roundToView(r.Round)}, nil
}
