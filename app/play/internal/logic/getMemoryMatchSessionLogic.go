package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMemoryMatchSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchSessionLogic {
	return &GetMemoryMatchSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMemoryMatchSessionLogic) GetMemoryMatchSession(in *play.GetMemoryMatchSessionReq) (*play.GetMemoryMatchSessionResp, error) {
	return gameimpl.NewGetMemoryMatchSessionLogic(l.ctx, l.svcCtx).GetMemoryMatchSession(in)
}
