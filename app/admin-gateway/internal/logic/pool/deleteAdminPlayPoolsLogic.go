package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAdminPlayPoolsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteAdminPlayPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAdminPlayPoolsLogic {
	return &DeleteAdminPlayPoolsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteAdminPlayPoolsLogic) DeleteAdminPlayPools(req *play.DeleteAdminPlayPoolsReq) (*play.DeleteAdminPlayPoolsResp, error) {
	return l.svcCtx.PlayRpc.DeleteAdminPlayPools(l.ctx, req)
}
