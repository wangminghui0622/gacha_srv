package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type SelectMemoryMatchDestinyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSelectMemoryMatchDestinyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SelectMemoryMatchDestinyLogic {
	return &SelectMemoryMatchDestinyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SelectMemoryMatchDestinyLogic) SelectMemoryMatchDestiny(in *play.SelectMemoryMatchDestinyReq) (*play.SelectMemoryMatchDestinyResp, error) {
	return gameimpl.NewSelectMemoryMatchDestinyLogic(l.ctx, l.svcCtx).SelectMemoryMatchDestiny(in)
}
