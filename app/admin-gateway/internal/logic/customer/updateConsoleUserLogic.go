// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateConsoleUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateConsoleUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateConsoleUserLogic {
	return &UpdateConsoleUserLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateConsoleUserLogic) UpdateConsoleUser(in *identity.UpdateConsoleUserReq) (*identity.ConsoleUser, error) {
	return l.svcCtx.IdentityRpc.UpdateConsoleUser(l.ctx, in)
}
