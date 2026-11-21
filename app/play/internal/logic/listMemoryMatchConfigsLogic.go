package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMemoryMatchConfigsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMemoryMatchConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemoryMatchConfigsLogic {
	return &ListMemoryMatchConfigsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListMemoryMatchConfigsLogic) ListMemoryMatchConfigs(in *play.ListMemoryMatchConfigsReq) (*play.ListMemoryMatchConfigsResp, error) {
	return gameimpl.NewListMemoryMatchConfigsLogic(l.ctx, l.svcCtx).ListMemoryMatchConfigs(in)
}
