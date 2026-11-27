package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type VoidPlayPoolLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVoidPlayPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VoidPlayPoolLogic {
	return &VoidPlayPoolLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VoidPlayPoolLogic) VoidPlayPool(in *play.VoidPlayPoolReq) (*play.VoidPlayPoolResp, error) {
	return gameimpl.VoidPlayPool(l.ctx, l.svcCtx, in)
}
