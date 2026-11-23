package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetirePlayPoolLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRetirePlayPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetirePlayPoolLogic {
	return &RetirePlayPoolLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RetirePlayPoolLogic) RetirePlayPool(in *play.RetirePlayPoolReq) (*play.RetirePlayPoolResp, error) {
	return gameimpl.RetirePlayPool(l.ctx, l.svcCtx, in)
}
