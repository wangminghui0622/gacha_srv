package logic

import (
	"context"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/utils/errorx"

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
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenantId 须为 1（单租户）")
	}
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID不能为空")
	}
	return buildConsoleUser(l.ctx, l.svcCtx, in.TenantId, in.UserId)
}
