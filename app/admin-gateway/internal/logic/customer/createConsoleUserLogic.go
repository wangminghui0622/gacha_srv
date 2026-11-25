// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateConsoleUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateConsoleUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateConsoleUserLogic {
	return &CreateConsoleUserLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreateConsoleUserLogic) CreateConsoleUser(in *identity.CreateConsoleUserReq) (*identity.ConsoleUser, error) {
	return l.svcCtx.IdentityRpc.CreateConsoleUser(l.ctx, in)
}
