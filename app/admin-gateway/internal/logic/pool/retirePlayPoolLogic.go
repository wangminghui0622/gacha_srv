// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetirePlayPoolLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetirePlayPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetirePlayPoolLogic {
	return &RetirePlayPoolLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// RetirePlayPool 下架赏池的所有已发布版本。
func (l *RetirePlayPoolLogic) RetirePlayPool(req *play.RetirePlayPoolReq) (*play.RetirePlayPoolResp, error) {
	return l.svcCtx.PlayRpc.RetirePlayPool(l.ctx, req)
}
