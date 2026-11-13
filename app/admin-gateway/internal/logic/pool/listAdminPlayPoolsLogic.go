package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAdminPlayPoolsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAdminPlayPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAdminPlayPoolsLogic {
	return &ListAdminPlayPoolsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListAdminPlayPoolsLogic) ListAdminPlayPools(req *play.ListAdminPlayPoolsReq) (*play.ListAdminPlayPoolsResp, error) {
	return l.svcCtx.PlayRpc.ListAdminPlayPools(l.ctx, req)
}
