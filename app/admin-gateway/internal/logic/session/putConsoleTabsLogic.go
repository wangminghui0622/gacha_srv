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

type PutConsoleTabsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPutConsoleTabsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutConsoleTabsLogic {
	return &PutConsoleTabsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PutConsoleTabsLogic) PutConsoleTabs(req *ops.PutConsoleTabsRequest) (*ops.ConsoleTabs, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话")
	}
	// 只允许覆盖自己的页签，请求体中的 tenant/account 一律忽略。
	req.TenantId = claims.TenantID
	req.AccountId = claims.ActorID
	return l.svcCtx.OpsRpc.PutConsoleTabs(l.ctx, req)
}
