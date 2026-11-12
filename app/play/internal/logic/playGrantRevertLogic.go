package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayGrantRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlayGrantRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayGrantRevertLogic {
	return &PlayGrantRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PlayGrantRevertLogic) PlayGrantRevert(in *play.PlayGrantReq) (*play.PlayGrantResp, error) {
	return doPlayGrant(l.ctx, l.svcCtx, in, true)
}
