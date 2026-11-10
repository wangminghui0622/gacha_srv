package logic

import (
	"context"
	"errors"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type OpenConsoleAccountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOpenConsoleAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenConsoleAccountLogic {
	return &OpenConsoleAccountLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OpenConsoleAccount 开通控制台账号（照抄 server_v2：不回传口令，TOTP 首登 setup；
// 回复 role_code 为 promotion-admin——server_v2 契约常量，角色绑定由 PutPrincipalRoles 完成）。
func (l *OpenConsoleAccountLogic) OpenConsoleAccount(in *ops.OpenConsoleAccountRequest) (*ops.OpenConsoleAccountReply, error) {
	tenantID := defaultTenant(in.TenantId)
	username := trimNonEmpty(in.Username)
	displayName := trimNonEmpty(in.DisplayName)
	if username == "" || len(username) > 64 || displayName == "" || len(displayName) > 128 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户名(1-64)/显示名(1-128)必填")
	}
	if len(in.Password) < 12 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "密码至少 12 字节")
	}

	_, err := l.svcCtx.AccountsModel.FindOneByTenantIdUsername(l.ctx, tenantID, username)
	if err == nil {
		return nil, errorx.NewCodeError(errorx.CodeConsoleAccountExists, "账号已存在")
	}
	if !repo.IsNotFound(err) {
		return nil, err
	}

	hash, err := core.DefaultArgon2idHasher().Hash(in.Password)
	if err != nil {
		return nil, err
	}
	account := &opsmodel.ConsoleAccounts{
		TenantId: tenantID, Username: username, PasswordHash: hash,
		DisplayName: displayName, Status: 1, RoleVersion: 1, TotpSecret: "",
	}
	if err = l.svcCtx.AccountsModel.Insert(l.ctx, nil, account); err != nil {
		// 并发兜底：唯一键冲突按账号已存在处理
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, errorx.NewCodeError(errorx.CodeConsoleAccountExists, "账号已存在")
		}
		return nil, err
	}
	return &ops.OpenConsoleAccountReply{
		PrincipalId: account.Id, TenantId: tenantID, Username: username,
		// server_v2 契约：开通者（promotion-admin）角色编码常量
		RoleCode: "promotion-admin",
	}, nil
}
