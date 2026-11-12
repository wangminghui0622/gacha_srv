package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type SelectMemoryMatchSecondDestinyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSelectMemoryMatchSecondDestinyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SelectMemoryMatchSecondDestinyLogic {
	return &SelectMemoryMatchSecondDestinyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SelectMemoryMatchSecondDestinyLogic) SelectMemoryMatchSecondDestiny(in *play.SelectMemoryMatchSecondDestinyReq) (*play.SelectMemoryMatchSecondDestinyResp, error) {
	return gameimpl.NewSelectMemoryMatchSecondDestinyLogic(l.ctx, l.svcCtx).SelectMemoryMatchSecondDestiny(in)
}
