package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchRoundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMemoryMatchRoundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchRoundLogic {
	return &GetMemoryMatchRoundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMemoryMatchRoundLogic) GetMemoryMatchRound(in *play.GetMemoryMatchRoundReq) (*play.GetMemoryMatchRoundResp, error) {
	return gameimpl.NewGetMemoryMatchRoundLogic(l.ctx, l.svcCtx).GetMemoryMatchRound(in)
}
