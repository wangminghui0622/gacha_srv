package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishPlayPoolLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishPlayPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishPlayPoolLogic {
	return &PublishPlayPoolLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PublishPlayPoolLogic) PublishPlayPool(in *play.PublishPlayPoolReq) (*play.PublishPlayPoolResp, error) {
	return gameimpl.PublishPlayPool(l.ctx, l.svcCtx, in)
}
