// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetUserBindingLimitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetUserBindingLimitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetUserBindingLimitLogic {
	return &ResetUserBindingLimitLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ResetUserBindingLimitLogic) ResetUserBindingLimit(in *identity.ResetUserBindingLimitReq) (*identity.ResetUserBindingLimitResp, error) {
	return l.svcCtx.IdentityRpc.ResetUserBindingLimit(l.ctx, in)
}
