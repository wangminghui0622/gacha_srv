package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayLockLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlayLockLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayLockLogic {
	return &PlayLockLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 通用 SAGA 分支（payload 内是各玩法 typed 请求）
func (l *PlayLockLogic) PlayLock(in *play.PlayLockReq) (*play.PlayLockResp, error) {
	return doPlayLock(l.ctx, l.svcCtx, in)
}
