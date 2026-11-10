// Package repo 用户域持久层（M15a）：手机号盲索引 / 第三方身份。
// 语义照抄 server_v2 identity data 层（mysql_phone / mysql_third_party）：
// exact_token 唯一索引仲裁并发绑定；搜索 501 候选上限（超限拒绝）；
// 每个候选解密验证真子串（防 gram 集合误报）。
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gacha_srv/app/identity/internal/phonecrypto"
	"gacha_srv/pkg/model_nocache/gacha_identity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxPhoneCandidates = 500

var (
	ErrPhoneInUse          = errors.New("phone already bound in tenant")
	ErrPhoneSearchTooBroad = errors.New("phone fragment matches too many candidates")
	ErrThirdPartyNotLinked = errors.New("third party identity is not linked")
)

type PhoneSearchResult struct {
	UserID      int64
	MaskedPhone string
}

type PhoneRepo struct {
	db    *gorm.DB
	codec *phonecrypto.Codec
}

func NewPhoneRepo(db *gorm.DB, codec *phonecrypto.Codec) *PhoneRepo {
	return &PhoneRepo{db: db, codec: codec}
}

// Set 事务内替换密文与全部搜索 token（同用户行锁串行化；跨用户由 exact_token
// 唯一索引仲裁，冲突 → ErrPhoneInUse）
func (r *PhoneRepo) Set(ctx context.Context, tenantID, userID int64, phone string) error {
	sealed, err := r.codec.Seal(tenantID, userID, phone)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p gacha_identity.UserPhones
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&p).Error
		now := time.Now().UTC()
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			err = tx.Create(&gacha_identity.UserPhones{
				TenantId:     tenantID,
				UserId:       userID,
				Ciphertext:   string(sealed.Ciphertext),
				Nonce:        string(sealed.Nonce),
				EncKeyId:     sealed.EncKeyID,
				IndexVersion: sealed.IndexVersion,
				ExactToken:   string(sealed.ExactToken[:]),
				CreatedAt:    now,
				UpdatedAt:    now,
			}).Error
		case err == nil:
			res := tx.Model(&gacha_identity.UserPhones{}).
				Where("tenant_id = ? AND user_id = ?", tenantID, userID).
				Updates(map[string]any{
					"ciphertext":    string(sealed.Ciphertext),
					"nonce":         string(sealed.Nonce),
					"enc_key_id":    sealed.EncKeyID,
					"index_version": sealed.IndexVersion,
					"exact_token":   string(sealed.ExactToken[:]),
					"updated_at":    now,
				})
			err = res.Error
			if err == nil && res.RowsAffected != 1 {
				return errors.New("phone update lost row")
			}
		}
		if err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrPhoneInUse
			}
			return fmt.Errorf("save user phone: %w", err)
		}
		if err = tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).
			Delete(&gacha_identity.UserPhoneGrams{}).Error; err != nil {
			return fmt.Errorf("delete old phone grams: %w", err)
		}
		for _, token := range sealed.GramTokens {
			if err = tx.Create(&gacha_identity.UserPhoneGrams{
				TenantId:     tenantID,
				IndexVersion: sealed.IndexVersion,
				GramToken:    string(token[:]),
				UserId:       userID,
			}).Error; err != nil {
				return fmt.Errorf("insert phone gram: %w", err)
			}
		}
		return nil
	})
}

// FindExact 精确查绑定（解密回验一致才返回）；未绑定 → gorm.ErrRecordNotFound
func (r *PhoneRepo) FindExact(ctx context.Context, tenantID int64, phone string) (int64, error) {
	token, err := r.codec.Exact(tenantID, phone)
	if err != nil {
		return 0, err
	}
	var p gacha_identity.UserPhones
	err = r.db.WithContext(ctx).
		Where("tenant_id = ? AND index_version = ? AND exact_token = ?",
			tenantID, r.codec.IndexVersion(), string(token[:])).First(&p).Error
	if err != nil {
		return 0, err
	}
	decrypted, err := r.codec.Open(tenantID, p.UserId, phonecrypto.Record{
		Ciphertext:   []byte(p.Ciphertext),
		Nonce:        []byte(p.Nonce),
		EncKeyID:     p.EncKeyId,
		IndexVersion: p.IndexVersion,
	})
	if err != nil {
		return 0, err
	}
	expected, _ := phonecrypto.NormalizeCNMobile(phone)
	if decrypted != expected {
		return 0, errors.New("phone exact index mismatch")
	}
	return p.UserId, nil
}

// Search 四位起连续片段搜索：gram 候选（501 上限，超限 → ErrPhoneSearchTooBroad）
// 逐个解密验证真子串后 3-4-4 掩码返回
func (r *PhoneRepo) Search(ctx context.Context, tenantID int64, fragment string) ([]PhoneSearchResult, error) {
	tokens, err := r.codec.SearchTokens(tenantID, fragment)
	if err != nil {
		return nil, err
	}
	var padded [8][]byte
	for i := range padded {
		padded[i] = tokens[0][:]
	}
	for i, token := range tokens {
		padded[i] = token[:]
	}
	args := make([]any, 0, 10)
	args = append(args, tenantID, r.codec.IndexVersion())
	for i := range padded {
		args = append(args, padded[i])
	}
	args = append(args, len(tokens), maxPhoneCandidates+1, tenantID)
	var rows []gacha_identity.UserPhones
	err = r.db.WithContext(ctx).Raw(`
SELECT p.* FROM (
  SELECT g.user_id FROM `+"`user_phone_grams`"+` g
  WHERE g.tenant_id=? AND g.index_version=? AND g.gram_token IN (?,?,?,?,?,?,?,?)
  GROUP BY g.user_id HAVING COUNT(DISTINCT g.gram_token)=?
  ORDER BY g.user_id LIMIT ?
) c JOIN `+"`user_phones`"+` p ON p.tenant_id=? AND p.user_id=c.user_id
ORDER BY p.user_id`, args...).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("search phone candidates: %w", err)
	}
	if len(rows) > maxPhoneCandidates {
		return nil, ErrPhoneSearchTooBroad
	}
	result := make([]PhoneSearchResult, 0, len(rows))
	for _, row := range rows {
		phone, openErr := r.codec.Open(tenantID, row.UserId, phonecrypto.Record{
			Ciphertext:   []byte(row.Ciphertext),
			Nonce:        []byte(row.Nonce),
			EncKeyID:     row.EncKeyId,
			IndexVersion: row.IndexVersion,
		})
		if openErr != nil {
			return nil, fmt.Errorf("verify phone candidate %d: %w", row.UserId, openErr)
		}
		if strings.Contains(phone, fragment) {
			result = append(result, PhoneSearchResult{UserID: row.UserId, MaskedPhone: phone[:3] + "****" + phone[7:]})
		}
	}
	return result, nil
}
