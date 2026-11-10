// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package session

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConsoleRefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 刷新令牌（旋转 + 指纹校验 + 重放家族吊销）
func NewConsoleRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConsoleRefreshLogic {
	return &ConsoleRefreshLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ConsoleRefreshLogic) ConsoleRefresh(req *ops.ConsoleRefreshRequest) (*ops.ConsoleSessionTokens, error) {
	return l.svcCtx.OpsRpc.ConsoleRefresh(l.ctx, req)
}
