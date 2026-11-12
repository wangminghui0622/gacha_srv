package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchSessionByRequestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMemoryMatchSessionByRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchSessionByRequestLogic {
	return &GetMemoryMatchSessionByRequestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMemoryMatchSessionByRequestLogic) GetMemoryMatchSessionByRequest(in *play.GetMemoryMatchSessionByRequestReq) (*play.GetMemoryMatchSessionByRequestResp, error) {
	return gameimpl.NewGetMemoryMatchSessionByRequestLogic(l.ctx, l.svcCtx).GetMemoryMatchSessionByRequest(in)
}
