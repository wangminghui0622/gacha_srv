// Package repo 运营服务数据访问：GORM 模型实现 core 包接口 + Redis 票据/吊销。
package repo

import (
	"context"
	"errors"

	"gacha_srv/app/ops/internal/core"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"

	"gorm.io/gorm"
)

// AccountStore 控制台账号读写（core.ConsoleAccountStore 实现）。
// 角色编码由 principal_roles + roles 联查实时解析（服务端权威，不信 JWT 快照）。
type AccountStore struct {
	db    *gorm.DB
	model opsmodel.ConsoleAccountsModel
}

// NewAccountStore 构造账号存储。
func NewAccountStore(db *gorm.DB) *AccountStore {
	return &AccountStore{db: db, model: opsmodel.NewConsoleAccountsModel(db)}
}

// FindByUsername 按租户+用户名查找（含角色编码）。
func (s *AccountStore) FindByUsername(ctx context.Context, tenantID uint64, username string) (*core.ConsoleAccount, error) {
	row, err := s.model.FindOneByTenantIdUsername(ctx, tenantID, username)
	if err != nil {
		return nil, err
	}
	return s.toAccount(ctx, row)
}

// FindByID 按租户+账号 ID 查找（含角色编码）。
func (s *AccountStore) FindByID(ctx context.Context, tenantID, accountID uint64) (*core.ConsoleAccount, error) {
	row, err := s.model.FindOne(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if row.TenantId != tenantID {
		return nil, opsmodel.ErrNotFound
	}
	return s.toAccount(ctx, row)
}

// EnableTotp 绑定 TOTP 密钥（登录 setup 流程验证通过后调用）。
func (s *AccountStore) EnableTotp(ctx context.Context, tenantID, accountID uint64, secret string) error {
	return s.db.WithContext(ctx).Model(&opsmodel.ConsoleAccounts{}).
		Where("`id` = ? AND `tenant_id` = ?", accountID, tenantID).
		Update("totp_secret", secret).Error
}

// ResetTotp 清空 TOTP 密钥（重新走 setup 流程）。
func (s *AccountStore) ResetTotp(ctx context.Context, tenantID, accountID uint64) error {
	return s.db.WithContext(ctx).Model(&opsmodel.ConsoleAccounts{}).
		Where("`id` = ? AND `tenant_id` = ?", accountID, tenantID).
		Update("totp_secret", "").Error
}

func (s *AccountStore) toAccount(ctx context.Context, row *opsmodel.ConsoleAccounts) (*core.ConsoleAccount, error) {
	roleCodes, err := s.roleCodes(ctx, row.Id)
	if err != nil {
		return nil, err
	}
	return &core.ConsoleAccount{
		ID: row.Id, TenantID: row.TenantId, Username: row.Username, DisplayName: row.DisplayName,
		PasswordHash: row.PasswordHash, RoleCodes: roleCodes, Disabled: row.Status != 1,
		TotpSecret: row.TotpSecret,
	}, nil
}

func (s *AccountStore) roleCodes(ctx context.Context, accountID uint64) ([]string, error) {
	var codes []string
	err := s.db.WithContext(ctx).
		Table("`principal_roles` AS pr").
		Select("r.code").
		Joins("JOIN `roles` AS r ON r.id = pr.role_id AND r.enabled = 1").
		Where("pr.account_id = ?", accountID).
		Order("r.id").
		Pluck("r.code", &codes).Error
	if err != nil {
		return nil, err
	}
	return codes, nil
}

// ErrNotFound 导出（逻辑层 errors.Is 判断用）。
var ErrNotFound = opsmodel.ErrNotFound

// IsNotFound 判断是否未找到。
func IsNotFound(err error) bool {
	return errors.Is(err, opsmodel.ErrNotFound)
}
