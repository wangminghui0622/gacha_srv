package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAdminPlayPoolsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteAdminPlayPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAdminPlayPoolsLogic {
	return &DeleteAdminPlayPoolsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *DeleteAdminPlayPoolsLogic) DeleteAdminPlayPools(in *play.DeleteAdminPlayPoolsReq) (*play.DeleteAdminPlayPoolsResp, error) {
	return gameimpl.DeleteAdminPlayPools(l.ctx, l.svcCtx, in)
}
