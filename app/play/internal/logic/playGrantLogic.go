package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayGrantLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlayGrantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayGrantLogic {
	return &PlayGrantLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PlayGrantLogic) PlayGrant(in *play.PlayGrantReq) (*play.PlayGrantResp, error) {
	return doPlayGrant(l.ctx, l.svcCtx, in, false)
}
