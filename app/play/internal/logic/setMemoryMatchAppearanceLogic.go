package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetMemoryMatchAppearanceLogic {
	return &SetMemoryMatchAppearanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetMemoryMatchAppearanceLogic) SetMemoryMatchAppearance(in *play.SetMemoryMatchAppearanceReq) (*play.SetMemoryMatchAppearanceResp, error) {
	return gameimpl.NewSetMemoryMatchAppearanceLogic(l.ctx, l.svcCtx).SetMemoryMatchAppearance(in)
}
