// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConsoleUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConsoleUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConsoleUserLogic {
	return &GetConsoleUserLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetConsoleUserLogic) GetConsoleUser(in *identity.GetConsoleUserReq) (*identity.ConsoleUser, error) {
	return l.svcCtx.IdentityRpc.GetConsoleUser(l.ctx, in)
}
