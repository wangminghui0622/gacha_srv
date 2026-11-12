package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlayPoolsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPlayPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlayPoolsLogic {
	return &ListPlayPoolsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListPlayPoolsLogic) ListPlayPools(in *play.ListPlayPoolsReq) (*play.ListPlayPoolsResp, error) {
	return gameimpl.ListPlayPools(l.ctx, l.svcCtx, in)
}
