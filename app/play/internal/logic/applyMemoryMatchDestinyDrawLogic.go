package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApplyMemoryMatchDestinyDrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplyMemoryMatchDestinyDrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyMemoryMatchDestinyDrawLogic {
	return &ApplyMemoryMatchDestinyDrawLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApplyMemoryMatchDestinyDrawLogic) ApplyMemoryMatchDestinyDraw(in *play.ApplyMemoryMatchDestinyDrawReq) (*play.ApplyMemoryMatchDestinyDrawResp, error) {
	return gameimpl.NewApplyMemoryMatchDestinyDrawLogic(l.ctx, l.svcCtx).ApplyMemoryMatchDestinyDraw(in)
}
