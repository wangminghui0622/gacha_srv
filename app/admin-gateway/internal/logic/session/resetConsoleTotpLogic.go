// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package session

import (
	"context"

	"gacha_srv/app/ops/ops"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetConsoleTotpLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重置账号 TOTP（identity.console_account:reset_totp）
func NewResetConsoleTotpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetConsoleTotpLogic {
	return &ResetConsoleTotpLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ResetConsoleTotpLogic) ResetConsoleTotp(req *ops.ResetConsoleTotpRequest) (*ops.ResetConsoleTotpReply, error) {
	return l.svcCtx.OpsRpc.ResetConsoleTotp(l.ctx, req)
}
