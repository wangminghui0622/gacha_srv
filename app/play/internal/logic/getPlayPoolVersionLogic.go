package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlayPoolVersionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPlayPoolVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlayPoolVersionLogic {
	return &GetPlayPoolVersionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPlayPoolVersionLogic) GetPlayPoolVersion(in *play.GetPlayPoolVersionReq) (*play.GetPlayPoolVersionResp, error) {
	return gameimpl.GetPlayPoolVersion(l.ctx, l.svcCtx, in)
}
