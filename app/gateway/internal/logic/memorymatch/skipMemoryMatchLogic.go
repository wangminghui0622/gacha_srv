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

type SkipMemoryMatchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSkipMemoryMatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SkipMemoryMatchLogic {
	return &SkipMemoryMatchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SkipMemoryMatch 跳过（POST .../skip；AutoResolve 剩余回合+终局结算+发奖；已跳过幂等重跑）
func (l *SkipMemoryMatchLogic) SkipMemoryMatch(sessionId int64) (resp *types.MemoryMatchSkipResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.SkipMemoryMatch(l.ctx, &playClient.SkipMemoryMatchReq{
		SessionId: sessionId,
		UserId:    userId,
	})
	if err != nil {
		l.Errorf("SkipMemoryMatch failed, sessionId: %d, err: %v", sessionId, err)
		return nil, errorx.FromError(err)
	}
	return &types.MemoryMatchSkipResp{
		SessionId:    r.SessionId,
		SessionState: int64(r.SessionState),
		Settlement:   settlementToView(r.Settlement),
	}, nil
}
