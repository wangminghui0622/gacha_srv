package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAdminPlayPoolLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAdminPlayPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAdminPlayPoolLogic {
	return &UpdateAdminPlayPoolLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateAdminPlayPoolLogic) UpdateAdminPlayPool(req *play.UpdateAdminPlayPoolReq) (*play.AdminPlayPool, error) {
	return l.svcCtx.PlayRpc.UpdateAdminPlayPool(l.ctx, req)
}
