package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrantMemoryMatchPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantMemoryMatchPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantMemoryMatchPrizesRevertLogic {
	return &GrantMemoryMatchPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantMemoryMatchPrizesRevert SAGA 06 补偿：空操作（正向即空操作，无奖可撤）。永不 Aborted。
func (l *GrantMemoryMatchPrizesRevertLogic) GrantMemoryMatchPrizesRevert(in *play.GrantMemoryMatchPrizesReq) (*play.GrantMemoryMatchPrizesResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}
	l.Infof("GrantMemoryMatchPrizesRevert no-op, requestId: %s, sessionId: %d", in.RequestId, in.SessionId)
	return &play.GrantMemoryMatchPrizesResp{Status: "NO_PRIZES_AT_PURCHASE"}, nil
}
