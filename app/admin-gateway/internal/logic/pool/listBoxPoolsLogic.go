// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/draw/draw"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBoxPoolsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListBoxPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBoxPoolsLogic {
	return &ListBoxPoolsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListBoxPoolsLogic) ListBoxPools(req *draw.ListBoxPoolsReq) (*draw.ListBoxPoolsResp, error) {
	return l.svcCtx.DrawRpc.ListBoxPools(l.ctx, req)
}
