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

type SelectMemoryMatchSecondDestinyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSelectMemoryMatchSecondDestinyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SelectMemoryMatchSecondDestinyLogic {
	return &SelectMemoryMatchSecondDestinyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SelectMemoryMatchSecondDestiny 选第二命运（POST .../second-destiny；未启用 3043、同值幂等、异值 6022）
func (l *SelectMemoryMatchSecondDestinyLogic) SelectMemoryMatchSecondDestiny(sessionId int64, patternId int32) (resp *types.MemoryMatchActionResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.SelectMemoryMatchSecondDestiny(l.ctx, &playClient.SelectMemoryMatchSecondDestinyReq{
		SessionId: sessionId,
		UserId:    userId,
		PatternId: patternId,
	})
	if err != nil {
		l.Errorf("SelectMemoryMatchSecondDestiny failed, sessionId: %d, err: %v", sessionId, err)
		return nil, errorx.FromError(err)
	}
	return &types.MemoryMatchActionResp{PatternId: r.PatternId, Status: r.Status}, nil
}
