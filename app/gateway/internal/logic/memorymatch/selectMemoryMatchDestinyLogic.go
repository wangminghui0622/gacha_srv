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

type SelectMemoryMatchDestinyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSelectMemoryMatchDestinyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SelectMemoryMatchDestinyLogic {
	return &SelectMemoryMatchDestinyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SelectMemoryMatchDestiny 选命运（POST /memory-match/sessions/:sessionId/destiny；同值幂等、异值 6022）
func (l *SelectMemoryMatchDestinyLogic) SelectMemoryMatchDestiny(sessionId int64, patternId int32) (resp *types.MemoryMatchActionResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.SelectMemoryMatchDestiny(l.ctx, &playClient.SelectMemoryMatchDestinyReq{
		SessionId: sessionId,
		UserId:    userId,
		PatternId: patternId,
	})
	if err != nil {
		l.Errorf("SelectMemoryMatchDestiny failed, sessionId: %d, err: %v", sessionId, err)
		return nil, errorx.FromError(err)
	}
	return &types.MemoryMatchActionResp{PatternId: r.PatternId, Status: r.Status}, nil
}
