// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package session

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConsolePermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 当前有效权限（免权限点，需会话）
func NewConsolePermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConsolePermissionsLogic {
	return &ConsolePermissionsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ConsolePermissionsLogic) ConsolePermissions() (*ops.ListEffectiveGrantsReply, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话")
	}
	return l.svcCtx.OpsRpc.ListEffectiveGrants(l.ctx, &ops.ListEffectiveGrantsRequest{
		ActorId:  claims.ActorID,
		TenantId: claims.TenantID,
		RoleIds:  claims.Roles,
	})
}
