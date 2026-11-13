package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAdminPlayPoolLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAdminPlayPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAdminPlayPoolLogic {
	return &UpdateAdminPlayPoolLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateAdminPlayPoolLogic) UpdateAdminPlayPool(in *play.UpdateAdminPlayPoolReq) (*play.AdminPlayPool, error) {
	return gameimpl.UpdateAdminPlayPool(l.ctx, l.svcCtx, in)
}
