// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package session

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConsoleLogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 退出登录（吊销指定会话）
func NewConsoleLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConsoleLogoutLogic {
	return &ConsoleLogoutLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ConsoleLogoutLogic) ConsoleLogout(req *ops.ConsoleLogoutRequest) (*ops.ConsoleLogoutReply, error) {
	return l.svcCtx.OpsRpc.ConsoleLogout(l.ctx, req)
}
