// Code scaffolded by goctl. Safe to edit.

package session

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConsoleTabsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetConsoleTabsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConsoleTabsLogic {
	return &GetConsoleTabsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetConsoleTabsLogic) GetConsoleTabs() (*ops.ConsoleTabs, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话")
	}
	return l.svcCtx.OpsRpc.GetConsoleTabs(l.ctx, &ops.GetConsoleTabsRequest{
		TenantId:  claims.TenantID,
		AccountId: claims.ActorID,
	})
}
