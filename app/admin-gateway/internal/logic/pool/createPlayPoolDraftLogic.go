// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlayPoolDraftLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlayPoolDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlayPoolDraftLogic {
	return &CreatePlayPoolDraftLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// CreatePlayPoolDraft 创建统一玩赏池草稿（幂等键由 handler 注入请求体）。
func (l *CreatePlayPoolDraftLogic) CreatePlayPoolDraft(req *play.CreatePlayPoolDraftReq) (*play.CreatePlayPoolDraftResp, error) {
	return l.svcCtx.PlayRpc.CreatePlayPoolDraft(l.ctx, req)
}
