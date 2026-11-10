// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package session

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenConsoleAccountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 开通控制台账号（identity.console_account:create）
func NewOpenConsoleAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenConsoleAccountLogic {
	return &OpenConsoleAccountLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *OpenConsoleAccountLogic) OpenConsoleAccount(req *ops.OpenConsoleAccountRequest) (*ops.OpenConsoleAccountReply, error) {
	return l.svcCtx.OpsRpc.OpenConsoleAccount(l.ctx, req)
}
