// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUserFinanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserFinanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserFinanceLogic {
	return &UpdateUserFinanceLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateUserFinanceLogic) UpdateUserFinance(in *identity.UpdateUserFinanceReq) (*identity.UserFinance, error) {
	return l.svcCtx.IdentityRpc.UpdateUserFinance(l.ctx, in)
}
