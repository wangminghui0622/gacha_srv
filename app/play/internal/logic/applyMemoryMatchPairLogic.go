package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApplyMemoryMatchPairLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplyMemoryMatchPairLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyMemoryMatchPairLogic {
	return &ApplyMemoryMatchPairLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApplyMemoryMatchPairLogic) ApplyMemoryMatchPair(in *play.ApplyMemoryMatchPairReq) (*play.ApplyMemoryMatchPairResp, error) {
	return gameimpl.NewApplyMemoryMatchPairLogic(l.ctx, l.svcCtx).ApplyMemoryMatchPair(in)
}
