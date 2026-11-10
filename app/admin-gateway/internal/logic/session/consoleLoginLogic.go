// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package session

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConsoleLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 控制台登录（TOTP 两步）
func NewConsoleLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConsoleLoginLogic {
	return &ConsoleLoginLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ConsoleLoginLogic) ConsoleLogin(req *ops.ConsoleLoginRequest) (*ops.ConsoleLoginReply, error) {
	return l.svcCtx.OpsRpc.ConsoleLogin(l.ctx, req)
}
