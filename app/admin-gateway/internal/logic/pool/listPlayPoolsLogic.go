// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlayPoolsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlayPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlayPoolsLogic {
	return &ListPlayPoolsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListPlayPoolsLogic) ListPlayPools(req *play.ListPlayPoolsReq) (*play.ListPlayPoolsResp, error) {
	return l.svcCtx.PlayRpc.ListPlayPools(l.ctx, req)
}
