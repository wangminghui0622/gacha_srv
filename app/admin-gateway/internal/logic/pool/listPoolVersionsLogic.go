// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPoolVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// ListPoolVersions 赏池版本列表（gacha.pool:read；游标分页，透传 draw-rpc）。
func NewListPoolVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPoolVersionsLogic {
	return &ListPoolVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPoolVersionsLogic) ListPoolVersions(req *draw.ListPoolVersionsReq) (*draw.ListPoolVersionsResp, error) {
	return l.svcCtx.DrawRpc.ListPoolVersions(l.ctx, req)
}
