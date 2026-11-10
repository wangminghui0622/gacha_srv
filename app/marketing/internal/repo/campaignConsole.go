// Package repo M14d 控制台数据访问（game 域）：活动版本化（campaigns=head +
// campaign_versions）+ 券模板不可变版本 + 发券 + 积分默认值。
// 事务由 logic 层开启并传入 tx；本包只做单表行级操作（M14b draw/repo/pool.go 先例）。
package repo

import (
	"time"

	"gorm.io/gorm"
)

// 表行（console 表无 goctl 模型——行锁 + CAS 场景直用 GORM）
type (
	// CampaignHead campaigns 表行（既是 head 也是 player 物化行）
	CampaignHead struct {
		ID               int64  `gorm:"column:id;primaryKey;autoIncrement"`
		CampaignKey      string `gorm:"column:campaign_key"`
		Name             string `gorm:"column:name"`
		Kind             int32  `gorm:"column:kind"`
		ConfigJSON       string `gorm:"column:config_json"`
		StartAt          time.Time
		EndAt            time.Time
		Status           int32 `gorm:"column:status"` // 0 草稿（玩家不可见）/1 已发布
		PublishedVersion int64 `gorm:"column:published_version"`
		CreatedAt        time.Time
		UpdatedAt        time.Time
	}

	// CampaignVersionRow campaign_versions 表行
	CampaignVersionRow struct {
		ID         int64 `gorm:"column:id;primaryKey;autoIncrement"`
		CampaignID int64 `gorm:"column:campaign_id"`
		Version    int64 `gorm:"column:version"`
		Kind       int32 `gorm:"column:kind"`
		Name       string
		State      int32 `gorm:"column:state"` // 1 DRAFT 2 PUBLISHED
		ConfigJSON string
		StartAt    time.Time
		EndAt      time.Time
		CreatedAt  time.Time
	}

	// RedemptionCodeRow campaign_redemption_codes 物化行
	RedemptionCodeRow struct {
		CampaignID     int64
		Code           string
		MaxRedemptions int64
		PerUserLimit   int64
		PrizeConfig    string // 奖品数组 JSON（[]playerPrize）
	}

	// WheelSlotRow campaign_wheel_slots 物化行（单奖品列=首奖品，玩家展示/兜底）
	WheelSlotRow struct {
		CampaignID       int64
		SlotIndex        int64
		Label            string
		Weight           int64
		PrizeKind        int64  // 首奖品 kind（1 CASH 2 POINTS 3 COUPON 4 GACHA_BOX）
		CashFen          int64  // 首奖品现金（分）
		PointsAmount     int64  // 首奖品积分
		CouponTemplateNo string // 首奖品券模板单号
		PrizeConfig      string // 全量奖品数组 JSON（非空=wheelTx 多奖品发放依据）
		DailyStock       int64  // -1 不限；>0 按日库存（跨日重置）
		Remaining        int64  // 总库存（console 槽 -1 不限）
	}
)

// 活动版本状态（与 server_v2 同值）
const (
	CampaignStateDraft     int32 = 1
	CampaignStatePublished int32 = 2
)

func (CampaignHead) TableName() string       { return "campaigns" }
func (CampaignVersionRow) TableName() string { return "campaign_versions" }

// GetCampaignHeadByKey 按 campaign_key 查 head；未找到返回 gorm.ErrRecordNotFound。
func GetCampaignHeadByKey(tx *gorm.DB, key string) (*CampaignHead, error) {
	var row CampaignHead
	if err := tx.Where("campaign_key = ?", key).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// LockCampaignHead 锁定 head 行（FOR UPDATE）；未找到返回 gorm.ErrRecordNotFound。
func LockCampaignHead(tx *gorm.DB, campaignID int64) (*CampaignHead, error) {
	var row CampaignHead
	res := tx.Raw(
		"SELECT id, campaign_key, name, kind, config_json, start_at, end_at, status, "+
			"published_version, created_at, updated_at FROM campaigns WHERE id = ? FOR UPDATE",
		campaignID).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// InsertCampaignHead 插入 head（新建草稿：status=0，published_version=0）。
func InsertCampaignHead(tx *gorm.DB, h *CampaignHead) error {
	return tx.Create(h).Error
}

// GetLatestCampaignVersion 读最新版本行（不加锁）；未找到返回 gorm.ErrRecordNotFound。
func GetLatestCampaignVersion(tx *gorm.DB, campaignID int64) (*CampaignVersionRow, error) {
	var row CampaignVersionRow
	if err := tx.Where("campaign_id = ?", campaignID).
		Order("version DESC").Limit(1).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// LockLatestCampaignVersion 锁定最新版本行（FOR UPDATE）；未找到返回 gorm.ErrRecordNotFound。
func LockLatestCampaignVersion(tx *gorm.DB, campaignID int64) (*CampaignVersionRow, error) {
	var row CampaignVersionRow
	res := tx.Raw(
		"SELECT id, campaign_id, version, kind, name, state, config_json, start_at, end_at, created_at "+
			"FROM campaign_versions WHERE campaign_id = ? ORDER BY version DESC LIMIT 1 FOR UPDATE",
		campaignID).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &row, nil
}

// InsertCampaignVersion 插入版本行（uk campaign_id+version 冲突返回 gorm.ErrDuplicatedKey）。
func InsertCampaignVersion(tx *gorm.DB, v *CampaignVersionRow) error {
	return tx.Create(v).Error
}

// GetCampaignVersion 读指定版本行；未找到返回 gorm.ErrRecordNotFound。
func GetCampaignVersion(tx *gorm.DB, campaignID, version int64) (*CampaignVersionRow, error) {
	var row CampaignVersionRow
	if err := tx.Where("campaign_id = ? AND version = ?", campaignID, version).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// PublishCampaignHeadCAS 物化发布到 head：期望 published_version 相符才更新
// （单事务串行 + 无并发争用，CAS 仅守卫重放）。返回 updated=false 表示期望不符。
func PublishCampaignHeadCAS(tx *gorm.DB, campaignID, expectedPublished, version int64,
	name, configJSON string, startAt, endAt time.Time) (bool, error) {
	res := tx.Exec(
		"UPDATE campaigns SET config_json = ?, name = ?, start_at = ?, end_at = ?, "+
			"status = 1, published_version = ? WHERE id = ? AND published_version = ?",
		configJSON, name, startAt, endAt, version, campaignID, expectedPublished)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// MarkCampaignVersionPublished 版本行置 PUBLISHED（期望 DRAFT）。
func MarkCampaignVersionPublished(tx *gorm.DB, campaignID, version int64) error {
	return tx.Exec(
		"UPDATE campaign_versions SET state = ? WHERE campaign_id = ? AND version = ? AND state = ?",
		CampaignStatePublished, campaignID, version, CampaignStateDraft).Error
}

// DisableRedemptionCodesExcept 停用本活动不在新配置中的兑换码（物化替换语义）。
func DisableRedemptionCodesExcept(tx *gorm.DB, campaignID int64, keep []string) error {
	if len(keep) == 0 {
		return tx.Exec(
			"UPDATE campaign_redemption_codes SET status = 0 WHERE campaign_id = ? AND status = 1",
			campaignID).Error
	}
	return tx.Exec(
		"UPDATE campaign_redemption_codes SET status = 0 "+
			"WHERE campaign_id = ? AND status = 1 AND code NOT IN (?)",
		campaignID, keep).Error
}

// UpsertRedemptionCode 物化兑换码（code 全局唯一；已存在则更新配置/上限/状态，
// redeemed_count 保留兑换进度）。
func UpsertRedemptionCode(tx *gorm.DB, r *RedemptionCodeRow) error {
	now := time.Now()
	return tx.Exec(
		"INSERT INTO campaign_redemption_codes "+
			"(campaign_id, code, max_redemptions, per_user_limit, prize_config, status, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, 1, ?, ?) "+
			"ON DUPLICATE KEY UPDATE campaign_id = VALUES(campaign_id), "+
			"max_redemptions = VALUES(max_redemptions), per_user_limit = VALUES(per_user_limit), "+
			"prize_config = VALUES(prize_config), status = 1, updated_at = VALUES(updated_at)",
		r.CampaignID, r.Code, r.MaxRedemptions, r.PerUserLimit, r.PrizeConfig, now, now).Error
}

// DeleteWheelSlots 清空活动转盘槽（物化替换：全删全插）。
func DeleteWheelSlots(tx *gorm.DB, campaignID int64) error {
	return tx.Exec("DELETE FROM campaign_wheel_slots WHERE campaign_id = ?", campaignID).Error
}

// InsertWheelSlot 插入转盘槽物化行。
func InsertWheelSlot(tx *gorm.DB, s *WheelSlotRow) error {
	return tx.Exec(
		"INSERT INTO campaign_wheel_slots "+
			"(campaign_id, slot_index, label, weight, prize_kind, cash_fen, points_amount, "+
			"coupon_template_no, prize_config, daily_stock, remaining, daily_spent, daily_stock_date, "+
			"created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NULL, ?, ?)",
		s.CampaignID, s.SlotIndex, s.Label, s.Weight, s.PrizeKind, s.CashFen, s.PointsAmount,
		s.CouponTemplateNo, s.PrizeConfig, s.DailyStock, s.Remaining, time.Now(), time.Now()).Error
}
