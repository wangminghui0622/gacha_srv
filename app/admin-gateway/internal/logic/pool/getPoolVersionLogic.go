// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPoolVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// GetPoolVersion 赏池版本详情（gacha.pool:read，透传 draw-rpc）。
func NewGetPoolVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPoolVersionLogic {
	return &GetPoolVersionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPoolVersionLogic) GetPoolVersion(req *draw.GetPoolVersionReq) (*draw.BoxDraft, error) {
	return l.svcCtx.DrawRpc.GetPoolVersion(l.ctx, req)
}
