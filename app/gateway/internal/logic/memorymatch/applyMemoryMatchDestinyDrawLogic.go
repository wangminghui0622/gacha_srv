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

type ApplyMemoryMatchDestinyDrawLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApplyMemoryMatchDestinyDrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyMemoryMatchDestinyDrawLogic {
	return &ApplyMemoryMatchDestinyDrawLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ApplyMemoryMatchDestinyDraw 命运抽牌（POST .../destiny-draw；有对子未配/资源耗尽 → 6025）
func (l *ApplyMemoryMatchDestinyDrawLogic) ApplyMemoryMatchDestinyDraw(sessionId, roundNo int64) (resp *types.MemoryMatchRoundActionResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.ApplyMemoryMatchDestinyDraw(l.ctx, &playClient.ApplyMemoryMatchDestinyDrawReq{
		SessionId: sessionId,
		UserId:    userId,
		RoundNo:   roundNo,
	})
	if err != nil {
		l.Errorf("ApplyMemoryMatchDestinyDraw failed, sessionId: %d, roundNo: %d, err: %v", sessionId, roundNo, err)
		return nil, errorx.FromError(err)
	}
	return &types.MemoryMatchRoundActionResp{Round: roundToView(r.Round)}, nil
}
