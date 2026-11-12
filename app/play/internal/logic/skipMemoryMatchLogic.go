package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type SkipMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSkipMemoryMatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SkipMemoryMatchLogic {
	return &SkipMemoryMatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SkipMemoryMatchLogic) SkipMemoryMatch(in *play.SkipMemoryMatchReq) (*play.SkipMemoryMatchResp, error) {
	return gameimpl.NewSkipMemoryMatchLogic(l.ctx, l.svcCtx).SkipMemoryMatch(in)
}
