// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlayPoolVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlayPoolVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlayPoolVersionLogic {
	return &GetPlayPoolVersionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetPlayPoolVersionLogic) GetPlayPoolVersion(req *play.GetPlayPoolVersionReq) (*play.GetPlayPoolVersionResp, error) {
	return l.svcCtx.PlayRpc.GetPlayPoolVersion(l.ctx, req)
}
