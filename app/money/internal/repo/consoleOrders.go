package repo

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RechargeOrder 充值订单表行（控制台列表/退款）。
type RechargeOrder struct {
	Id                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	OrderNo            string     `gorm:"column:order_no"`
	UserId             int64      `gorm:"column:user_id"`
	UserNickname       string     `gorm:"column:user_nickname"`
	AmountFen          int64      `gorm:"column:amount_fen"`
	CreditedFen        int64      `gorm:"column:credited_fen"`
	DiscountFen        int64      `gorm:"column:discount_fen"`
	Platform           string     `gorm:"column:platform"`
	Status             int32      `gorm:"column:status"`
	RechargeType       string     `gorm:"column:recharge_type"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	CompletedAt        *time.Time `gorm:"column:completed_at"`
	RefundedAt         *time.Time `gorm:"column:refunded_at"`
	RefundAmountFen    int64      `gorm:"column:refund_amount_fen"`
	RefundOrderNo      string     `gorm:"column:refund_order_no"`
	PayPlatformOrderId string     `gorm:"column:pay_platform_order_id"`
	PayPlatformOpenid  string     `gorm:"column:pay_platform_openid"`
	ErrorMessage       string     `gorm:"column:error_message"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (RechargeOrder) TableName() string { return "recharge_orders" }

// RefundOrder 退款订单表行（控制台审核/异常处理）。
type RefundOrder struct {
	Id                int64      `gorm:"column:id;primaryKey;autoIncrement"`
	OrderNo           string     `gorm:"column:order_no"`
	UserId            int64      `gorm:"column:user_id"`
	UserNickname      string     `gorm:"column:user_nickname"`
	RealName          string     `gorm:"column:real_name"`
	AlipayAccount     string     `gorm:"column:alipay_account"`
	AmountFen         int64      `gorm:"column:amount_fen"`
	Status            int32      `gorm:"column:status"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	CompletedAt       *time.Time `gorm:"column:completed_at"`
	ThirdPartyOrderId string     `gorm:"column:third_party_order_id"`
	RefundMerchantId  string     `gorm:"column:refund_merchant_id"`
	Auditor           string     `gorm:"column:auditor"`
	FinanceAuditor    string     `gorm:"column:finance_auditor"`
	AuditRemark       string     `gorm:"column:audit_remark"`
	ErrorMessage      string     `gorm:"column:error_message"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (RefundOrder) TableName() string { return "refund_orders" }

// RefundAutoConfig 自动退款配置（单行 id=1）。
type RefundAutoConfig struct {
	Id                 int64     `gorm:"column:id;primaryKey"`
	RemainingAmountFen int64     `gorm:"column:remaining_amount_fen"`
	AuditLimitFen      int64     `gorm:"column:audit_limit_fen"`
	DelaySeconds       int64     `gorm:"column:delay_seconds"`
	AlternateCount     int32     `gorm:"column:alternate_count"`
	AlternateAmountFen int64     `gorm:"column:alternate_amount_fen"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (RefundAutoConfig) TableName() string { return "refund_auto_configs" }

type RechargeOrderFilter struct {
	Keyword      string
	Status       int32
	RechargeType string
	Platform     string
	PayKeyword   string
	MinAmountFen int64
	MaxAmountFen int64
	TimeField    string
	StartAt      *time.Time
	EndAt        *time.Time
	Page         int
	PageSize     int
}

// ListRechargeOrders 充值订单分页查询。
func ListRechargeOrders(db *gorm.DB, f RechargeOrderFilter) ([]RechargeOrder, int64, error) {
	q := db.Model(&RechargeOrder{})
	if f.Keyword != "" {
		if userId, err := strconv.ParseInt(f.Keyword, 10, 64); err == nil && userId > 0 {
			q = q.Where("order_no LIKE ? OR user_id = ?", "%"+f.Keyword+"%", userId)
		} else {
			q = q.Where("order_no LIKE ?", "%"+f.Keyword+"%")
		}
	}
	if f.Status != 0 {
		q = q.Where("status = ?", f.Status)
	}
	if f.RechargeType != "" {
		q = q.Where("recharge_type = ?", f.RechargeType)
	}
	if f.Platform != "" {
		q = q.Where("platform = ?", f.Platform)
	}
	if f.PayKeyword != "" {
		q = q.Where("pay_platform_order_id LIKE ? OR pay_platform_openid LIKE ? OR refund_order_no LIKE ?",
			"%"+f.PayKeyword+"%", "%"+f.PayKeyword+"%", "%"+f.PayKeyword+"%")
	}
	if f.MinAmountFen > 0 {
		q = q.Where("amount_fen >= ?", f.MinAmountFen)
	}
	if f.MaxAmountFen > 0 {
		q = q.Where("amount_fen <= ?", f.MaxAmountFen)
	}
	if f.StartAt != nil {
		q = q.Where(timeColumn(f.TimeField, "created_at")+" >= ?", *f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where(timeColumn(f.TimeField, "created_at")+" <= ?", *f.EndAt)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []RechargeOrder
	if err := q.Order("id DESC").Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetRechargeOrder 按主键读取充值订单。
func GetRechargeOrder(db *gorm.DB, id int64) (*RechargeOrder, error) {
	var row RechargeOrder
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// UpdateRechargeOrder 更新充值订单字段。
func UpdateRechargeOrder(db *gorm.DB, id int64, updates map[string]any) error {
	return db.Model(&RechargeOrder{}).Where("id = ?", id).Updates(updates).Error
}

type RefundOrderFilter struct {
	Keyword      string
	AuditRemark  string
	Status       int32
	MinAmountFen int64
	MaxAmountFen int64
	TimeField    string
	StartAt      *time.Time
	EndAt        *time.Time
	Page         int
	PageSize     int
}

// ListRefundOrders 退款订单分页查询。
func ListRefundOrders(db *gorm.DB, f RefundOrderFilter) ([]RefundOrder, int64, error) {
	q := db.Model(&RefundOrder{})
	if f.Keyword != "" {
		if userId, err := strconv.ParseInt(f.Keyword, 10, 64); err == nil && userId > 0 {
			q = q.Where("order_no LIKE ? OR user_id = ? OR alipay_account LIKE ? OR real_name LIKE ?",
				"%"+f.Keyword+"%", userId, "%"+f.Keyword+"%", "%"+f.Keyword+"%")
		} else {
			q = q.Where("order_no LIKE ? OR alipay_account LIKE ? OR real_name LIKE ?",
				"%"+f.Keyword+"%", "%"+f.Keyword+"%", "%"+f.Keyword+"%")
		}
	}
	if f.AuditRemark != "" {
		q = q.Where("audit_remark LIKE ?", "%"+f.AuditRemark+"%")
	}
	if f.Status != 0 {
		q = q.Where("status = ?", f.Status)
	}
	if f.MinAmountFen > 0 {
		q = q.Where("amount_fen >= ?", f.MinAmountFen)
	}
	if f.MaxAmountFen > 0 {
		q = q.Where("amount_fen <= ?", f.MaxAmountFen)
	}
	if f.StartAt != nil {
		q = q.Where(timeColumn(f.TimeField, "created_at")+" >= ?", *f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where(timeColumn(f.TimeField, "created_at")+" <= ?", *f.EndAt)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []RefundOrder
	if err := q.Order("id DESC").Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetRefundOrder 按主键读取退款订单。
func GetRefundOrder(db *gorm.DB, id int64) (*RefundOrder, error) {
	var row RefundOrder
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// UpdateRefundOrder 更新退款订单字段。
func UpdateRefundOrder(db *gorm.DB, id int64, updates map[string]any) error {
	return db.Model(&RefundOrder{}).Where("id = ?", id).Updates(updates).Error
}

// GetRefundAutoConfig 读取自动退款配置（无行时返回零值配置）。
func GetRefundAutoConfig(db *gorm.DB) (*RefundAutoConfig, error) {
	var row RefundAutoConfig
	err := db.Where("id = 1").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &RefundAutoConfig{Id: 1}, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// SaveRefundAutoConfig 保存自动退款配置（upsert 单行）。
func SaveRefundAutoConfig(db *gorm.DB, c *RefundAutoConfig) error {
	c.Id = 1
	return db.Save(c).Error
}

func timeColumn(field, fallback string) string {
	switch strings.TrimSpace(field) {
	case "completed":
		return "completed_at"
	case "refunded":
		return "refunded_at"
	case "created":
		return "created_at"
	default:
		return fallback
	}
}
