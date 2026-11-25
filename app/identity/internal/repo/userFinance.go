package repo

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserFinanceSettings 用户财务设置（钱包开关、退款上限、限制重置时间）。
type UserFinanceSettings struct {
	ID                   int64      `gorm:"column:id;primaryKey;autoIncrement"`
	UserID               int64      `gorm:"column:user_id"`
	WalletSettingsJSON   string     `gorm:"column:wallet_settings_json"`
	RefundLimitFen       int64      `gorm:"column:refund_limit_fen"`
	RefundLimitCount     int32      `gorm:"column:refund_limit_count"`
	SMSLimitResetAt      *time.Time `gorm:"column:sms_limit_reset_at"`
	RealnameLimitResetAt *time.Time `gorm:"column:realname_limit_reset_at"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at"`
}

func (UserFinanceSettings) TableName() string { return "user_finance_settings" }

type PaymentBinding struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID         int64     `gorm:"column:user_id"`
	Name           string    `gorm:"column:name"`
	PaymentAccount string    `gorm:"column:payment_account"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (PaymentBinding) TableName() string { return "user_payment_bindings" }

type RealnameBinding struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64     `gorm:"column:user_id"`
	Name      string    `gorm:"column:name"`
	IDCard    string    `gorm:"column:id_card"`
	Phone     string    `gorm:"column:phone"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (RealnameBinding) TableName() string { return "user_realname_bindings" }

func GetUserFinanceSettings(db *gorm.DB, userID int64) (*UserFinanceSettings, error) {
	var row UserFinanceSettings
	if err := db.Where("user_id = ?", userID).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func SaveUserFinanceSettings(db *gorm.DB, row *UserFinanceSettings) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"wallet_settings_json", "refund_limit_fen", "refund_limit_count",
			"sms_limit_reset_at", "realname_limit_reset_at", "updated_at",
		}),
	}).Create(row).Error
}

func ListPaymentBindings(db *gorm.DB, userID int64) ([]PaymentBinding, error) {
	var rows []PaymentBinding
	if err := db.Where("user_id = ?", userID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func ListRealnameBindings(db *gorm.DB, userID int64) ([]RealnameBinding, error) {
	var rows []RealnameBinding
	if err := db.Where("user_id = ?", userID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func UnmarshalWalletSettings(raw string) map[string]bool {
	values := map[string]bool{}
	if raw == "" {
		return values
	}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func MarshalWalletSettings(values map[string]bool) string {
	if values == nil {
		return "{}"
	}
	b, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(b)
}
