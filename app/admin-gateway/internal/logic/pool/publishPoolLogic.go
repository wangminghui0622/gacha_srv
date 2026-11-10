// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishPoolLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// PublishPool 发布赏池（gacha.pool:publish，透传 draw-rpc）。
func NewPublishPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishPoolLogic {
	return &PublishPoolLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishPoolLogic) PublishPool(req *draw.PublishPoolReq) (*draw.BoxDraft, error) {
	return l.svcCtx.DrawRpc.PublishPool(l.ctx, req)
}
