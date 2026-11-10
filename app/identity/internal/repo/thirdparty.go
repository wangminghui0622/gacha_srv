package repo

import (
	"context"
	"errors"

	"gacha_srv/pkg/model_nocache/gacha_identity"

	"gorm.io/gorm"
)

type ThirdPartyRepo struct {
	db *gorm.DB
}

func NewThirdPartyRepo(db *gorm.DB) *ThirdPartyRepo {
	return &ThirdPartyRepo{db: db}
}

// Link 幂等 Upsert（uk tenant+kind+open_id 冲突时改绑 user/union——server_v2 语义）
func (r *ThirdPartyRepo) Link(ctx context.Context, row *gacha_identity.UserThirdParties) error {
	return r.db.WithContext(ctx).Exec(`
INSERT INTO `+"`user_third_parties`"+`(tenant_id,kind,open_id,union_id,user_id,created_at)
VALUES(?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE union_id=VALUES(union_id),user_id=VALUES(user_id)`,
		row.TenantId, row.Kind, row.OpenId, row.UnionId, row.UserId, row.CreatedAt).Error
}

// ResolveUser 按 openid 解析用户；未绑定 → ErrThirdPartyNotLinked
func (r *ThirdPartyRepo) ResolveUser(ctx context.Context, tenantID, kind int64, openID string) (int64, error) {
	var row gacha_identity.UserThirdParties
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND kind = ? AND open_id = ?", tenantID, kind, openID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrThirdPartyNotLinked
	}
	if err != nil {
		return 0, err
	}
	return row.UserId, nil
}

// GetByUser 查用户某类第三方身份；未绑定 → ErrThirdPartyNotLinked
func (r *ThirdPartyRepo) GetByUser(ctx context.Context, tenantID, userID, kind int64) (*gacha_identity.UserThirdParties, error) {
	var row gacha_identity.UserThirdParties
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND kind = ? AND user_id = ?", tenantID, kind, userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrThirdPartyNotLinked
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
