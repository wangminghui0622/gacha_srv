// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListConsoleUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListConsoleUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListConsoleUsersLogic {
	return &ListConsoleUsersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListConsoleUsersLogic) ListConsoleUsers(in *identity.ListConsoleUsersReq) (*identity.ListConsoleUsersResp, error) {
	return l.svcCtx.IdentityRpc.ListConsoleUsers(l.ctx, in)
}
