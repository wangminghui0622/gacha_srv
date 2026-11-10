// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePoolDraftLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// CreatePoolDraft 创建赏池草稿（gacha.pool:create；幂等键已由 handler 从
// Idempotency-Key 头注入，透传 draw-rpc）。
func NewCreatePoolDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePoolDraftLogic {
	return &CreatePoolDraftLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePoolDraftLogic) CreatePoolDraft(req *draw.CreatePoolDraftReq) (*draw.BoxDraft, error) {
	return l.svcCtx.DrawRpc.CreatePoolDraft(l.ctx, req)
}
