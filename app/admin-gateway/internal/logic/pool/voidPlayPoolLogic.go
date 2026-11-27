// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type VoidPlayPoolLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewVoidPlayPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VoidPlayPoolLogic {
	return &VoidPlayPoolLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// VoidPlayPool 作废赏池，不可恢复。
func (l *VoidPlayPoolLogic) VoidPlayPool(req *play.VoidPlayPoolReq) (*play.VoidPlayPoolResp, error) {
	return l.svcCtx.PlayRpc.VoidPlayPool(l.ctx, req)
}
