// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserBindingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserBindingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserBindingsLogic {
	return &GetUserBindingsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserBindingsLogic) GetUserBindings(in *identity.GetUserBindingsReq) (*identity.UserBindings, error) {
	return l.svcCtx.IdentityRpc.GetUserBindings(l.ctx, in)
}
