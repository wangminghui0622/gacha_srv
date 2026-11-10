package repo

import (
	"context"
	"fmt"
	"time"

	"gacha_srv/app/ops/internal/core"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"

	"gorm.io/gorm"
)

// SessionStore 控制台会话读写（core.ConsoleSessionStore 实现）。
type SessionStore struct {
	db    *gorm.DB
	model opsmodel.ConsoleSessionsModel
}

// NewSessionStore 构造会话存储。
func NewSessionStore(db *gorm.DB) *SessionStore {
	return &SessionStore{db: db, model: opsmodel.NewConsoleSessionsModel(db)}
}

// Create 新建会话行。
func (s *SessionStore) Create(ctx context.Context, session *core.ConsoleSession) error {
	return s.model.Insert(ctx, nil, &opsmodel.ConsoleSessions{
		Jti: session.Jti, FamilyId: session.FamilyID, AccountId: session.AccountID, TenantId: session.TenantID,
		RefreshHash: session.RefreshHash, PreviousRefreshHash: "", FingerprintHash: session.FingerprintHash,
		ExpiresAt: session.ExpiresAt, RevokedAt: nil, Compromised: 0,
	})
}

// FindByRefreshHash 按 refresh 哈希查会话（含上一枚哈希）。
// matchedViaPrevious=true 表示重放已旋转的旧令牌。
func (s *SessionStore) FindByRefreshHash(ctx context.Context, hash string) (*core.ConsoleSession, bool, error) {
	var row opsmodel.ConsoleSessions
	err := s.db.WithContext(ctx).Model(&opsmodel.ConsoleSessions{}).
		Where("`refresh_hash` = ? OR `previous_refresh_hash` = ?", hash, hash).
		Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, opsmodel.ErrNotFound
	}
	if err != nil {
		return nil, false, err
	}
	return toSession(&row), row.RefreshHash != hash, nil
}

// Rotate 原子旋转 refresh 哈希（旧哈希守卫防并发双花；旧哈希存 previous 供重放检测，滑动续期）。
func (s *SessionStore) Rotate(ctx context.Context, jti, oldHash, newHash string, newExpiresAt time.Time) error {
	res := s.db.WithContext(ctx).Model(&opsmodel.ConsoleSessions{}).
		Where("`jti` = ? AND `refresh_hash` = ? AND `revoked_at` IS NULL", jti, oldHash).
		Updates(map[string]any{
			"refresh_hash": newHash, "previous_refresh_hash": oldHash, "expires_at": newExpiresAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("会话旋转失败：旧哈希不匹配或已吊销")
	}
	return nil
}

// Revoke 吊销单会话（幂等：已吊销返回 nil）。
func (s *SessionStore) Revoke(ctx context.Context, jti string, compromised bool) error {
	updates := map[string]any{"revoked_at": time.Now()}
	if compromised {
		updates["compromised"] = 1
	}
	return s.db.WithContext(ctx).Model(&opsmodel.ConsoleSessions{}).
		Where("`jti` = ? AND `revoked_at` IS NULL", jti).
		Updates(updates).Error
}

// RevokeActiveByAccount 吊销账号全部活跃会话，返回被吊销的会话 ID 列表。
func (s *SessionStore) RevokeActiveByAccount(ctx context.Context, tenantID, accountID uint64) ([]string, error) {
	var jtis []string
	err := s.db.WithContext(ctx).Model(&opsmodel.ConsoleSessions{}).
		Where("`tenant_id` = ? AND `account_id` = ? AND `revoked_at` IS NULL", tenantID, accountID).
		Pluck("jti", &jtis).Error
	if err != nil {
		return nil, err
	}
	if len(jtis) == 0 {
		return nil, nil
	}
	err = s.db.WithContext(ctx).Model(&opsmodel.ConsoleSessions{}).
		Where("`jti` IN ? AND `revoked_at` IS NULL", jtis).
		Update("revoked_at", time.Now()).Error
	if err != nil {
		return nil, err
	}
	return jtis, nil
}

func toSession(row *opsmodel.ConsoleSessions) *core.ConsoleSession {
	return &core.ConsoleSession{
		Jti: row.Jti, FamilyID: row.FamilyId, AccountID: row.AccountId, TenantID: row.TenantId,
		RefreshHash: row.RefreshHash, FingerprintHash: row.FingerprintHash,
		ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt, Compromised: row.Compromised != 0,
	}
}
