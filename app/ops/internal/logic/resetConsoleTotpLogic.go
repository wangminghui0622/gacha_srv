package logic

import (
	"context"

	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetConsoleTotpLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetConsoleTotpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetConsoleTotpLogic {
	return &ResetConsoleTotpLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResetConsoleTotp 重置 TOTP（清空密钥，下次登录重走 setup）并吊销该账号全部活跃会话。
func (l *ResetConsoleTotpLogic) ResetConsoleTotp(in *ops.ResetConsoleTotpRequest) (*ops.ResetConsoleTotpReply, error) {
	tenantID := defaultTenant(in.TenantId)
	if in.PrincipalId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "账号 ID 必填")
	}
	store := repo.NewAccountStore(l.svcCtx.DB)
	if _, err := store.FindByID(l.ctx, tenantID, in.PrincipalId); err != nil {
		if repo.IsNotFound(err) {
			return nil, errorx.NewCodeError(errorx.CodeConsoleAccountNotFound, "账号不存在")
		}
		return nil, err
	}
	if err := store.ResetTotp(l.ctx, tenantID, in.PrincipalId); err != nil {
		return nil, err
	}
	revoked, err := l.svcCtx.SessionMgr.RevokeAccountSessions(l.ctx, tenantID, in.PrincipalId)
	if err != nil {
		return nil, err
	}
	return &ops.ResetConsoleTotpReply{Reset_: true, SessionsRevoked: revoked}, nil
}
