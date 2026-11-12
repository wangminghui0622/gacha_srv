package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayReleaseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlayReleaseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayReleaseLogic {
	return &PlayReleaseLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PlayReleaseLogic) PlayRelease(in *play.PlayLockReq) (*play.PlayReleaseResp, error) {
	return doPlayRelease(l.ctx, l.svcCtx, in)
}
