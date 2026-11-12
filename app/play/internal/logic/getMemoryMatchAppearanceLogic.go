package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchAppearanceLogic {
	return &GetMemoryMatchAppearanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMemoryMatchAppearanceLogic) GetMemoryMatchAppearance(in *play.GetMemoryMatchAppearanceReq) (*play.GetMemoryMatchAppearanceResp, error) {
	return gameimpl.NewGetMemoryMatchAppearanceLogic(l.ctx, l.svcCtx).GetMemoryMatchAppearance(in)
}
