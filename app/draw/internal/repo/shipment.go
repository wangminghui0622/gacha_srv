package repo

import (
	"time"

	"gorm.io/gorm"
)

// ShipmentOrder 发货订单表行（后台发货订单列表/编辑）。
type ShipmentOrder struct {
	Id               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	OrderNo          string     `gorm:"column:order_no"`
	UserId           int64      `gorm:"column:user_id"`
	Status           int32      `gorm:"column:status"`
	ReceiverName     string     `gorm:"column:receiver_name"`
	ReceiverPhone    string     `gorm:"column:receiver_phone"`
	ReceiverAddress  string     `gorm:"column:receiver_address"`
	LogisticsCompany string     `gorm:"column:logistics_company"`
	TrackingNo       string     `gorm:"column:tracking_no"`
	Remark           string     `gorm:"column:remark"`
	PaidAt           *time.Time `gorm:"column:paid_at"`
	SubmittedAt      *time.Time `gorm:"column:submitted_at"`
	ShippedAt        *time.Time `gorm:"column:shipped_at"`
	CompletedAt      *time.Time `gorm:"column:completed_at"`
	ClosedAt         *time.Time `gorm:"column:closed_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (ShipmentOrder) TableName() string { return "shipment_orders" }

// ShipmentOrderItem 发货订单物品行（吉仓物品快照）。
type ShipmentOrderItem struct {
	Id              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	OrderId         int64     `gorm:"column:order_id"`
	WarehouseItemId int64     `gorm:"column:warehouse_item_id"`
	BoxId           int64     `gorm:"column:box_id"`
	ItemName        string    `gorm:"column:item_name"`
	ImageUrl        string    `gorm:"column:image_url"`
	Quantity        int32     `gorm:"column:quantity"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (ShipmentOrderItem) TableName() string { return "shipment_order_items" }

// ListShipmentOrders 发货订单分页（keyword 支持订单号/用户ID，status=0 全部）。
func ListShipmentOrders(db *gorm.DB, keyword string, status int32, offset, limit int) ([]ShipmentOrder, int64, error) {
	q := db.Model(&ShipmentOrder{})
	if keyword != "" {
		// 调用方已把纯数字关键字同时按用户ID匹配（见 logic 层）。
		q = q.Where("order_no LIKE ?", "%"+keyword+"%")
	}
	if status != 0 {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ShipmentOrder
	if err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// ListShipmentOrdersByUserId 在订单号模糊匹配之外补用户ID精确匹配。
func ListShipmentOrdersByUserId(db *gorm.DB, keyword string, userId int64, status int32, offset, limit int) ([]ShipmentOrder, int64, error) {
	q := db.Model(&ShipmentOrder{}).
		Where("order_no LIKE ? OR user_id = ?", "%"+keyword+"%", userId)
	if status != 0 {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ShipmentOrder
	if err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetShipmentOrder 按主键读取发货订单。
func GetShipmentOrder(db *gorm.DB, id int64) (*ShipmentOrder, error) {
	var row ShipmentOrder
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ListShipmentOrderItems 批量读取订单物品（按订单ID）。
func ListShipmentOrderItems(db *gorm.DB, orderIDs []int64) ([]ShipmentOrderItem, error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}
	var rows []ShipmentOrderItem
	if err := db.Where("order_id IN ?", orderIDs).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListShipmentOrderItemsByOrder 读取单个订单物品。
func ListShipmentOrderItemsByOrder(db *gorm.DB, orderID int64) ([]ShipmentOrderItem, error) {
	var rows []ShipmentOrderItem
	if err := db.Where("order_id = ?", orderID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdateShipmentOrder 更新订单后台可编辑字段。
func UpdateShipmentOrder(db *gorm.DB, id int64, updates map[string]any) error {
	return db.Model(&ShipmentOrder{}).Where("id = ?", id).Updates(updates).Error
}
