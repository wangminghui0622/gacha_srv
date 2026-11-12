package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteMemoryMatchRoundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteMemoryMatchRoundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteMemoryMatchRoundLogic {
	return &CompleteMemoryMatchRoundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CompleteMemoryMatchRoundLogic) CompleteMemoryMatchRound(in *play.CompleteMemoryMatchRoundReq) (*play.CompleteMemoryMatchRoundResp, error) {
	return gameimpl.NewCompleteMemoryMatchRoundLogic(l.ctx, l.svcCtx).CompleteMemoryMatchRound(in)
}
