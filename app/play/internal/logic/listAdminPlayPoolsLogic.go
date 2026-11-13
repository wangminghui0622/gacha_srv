package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAdminPlayPoolsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAdminPlayPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAdminPlayPoolsLogic {
	return &ListAdminPlayPoolsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListAdminPlayPoolsLogic) ListAdminPlayPools(in *play.ListAdminPlayPoolsReq) (*play.ListAdminPlayPoolsResp, error) {
	return gameimpl.ListAdminPlayPools(l.ctx, l.svcCtx, in)
}
