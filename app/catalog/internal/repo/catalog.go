// Package repo 商品目录数据访问（M14c）：七表 GORM 行结构 + 单库事务流。
// 事务语义逐条移植 server_v2 catalog data/mysql.go + revision.go：
//
//	live 表=草稿工作区；发布=激活+落快照；已发布版本只读快照；
//	修订=草稿 blob（base/target 校验 + 同内容幂等）；修订发布=版本前进+删草稿。
//
// 本包只做数据流；业务校验在 core；行锁/CAS 在事务内。
package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	catalogpb "gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"

	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 业务哨兵错误（logic 层映射 errorx 4001/4002）
var (
	ErrNotFound      = errors.New("product not found")
	ErrConflict      = errors.New("product version or status conflict")
	ErrIPExists      = errors.New("ip name already exists")
	ErrBrandExists   = errors.New("brand name already exists")
	ErrSeriesExists  = errors.New("series name already exists in brand")
	ErrBrandNotFound = errors.New("brand not found")
)

// 状态取值（与 proto ProductStatus 同值）
const (
	StatusDraft    uint8 = 1
	StatusActive   uint8 = 2
	StatusArchived uint8 = 3
)

// ── 表行（nocache，无 goctl 模型——管理侧低频读写 + 行锁/CAS 直用 GORM）──

type productRow struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	ProductCode string    `gorm:"column:product_code"`
	Title       string    `gorm:"column:title"`
	Status      uint8     `gorm:"column:status"`
	Version     uint64    `gorm:"column:version"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (productRow) TableName() string { return "products" }

type ipRow struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Name       string    `gorm:"column:name"`
	SortWeight int32     `gorm:"column:sort_weight"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (ipRow) TableName() string { return "catalog_ips" }

type brandRow struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string    `gorm:"column:name"`
	Logo      string    `gorm:"column:logo"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (brandRow) TableName() string { return "catalog_brands" }

type seriesRow struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Name       string    `gorm:"column:name"`
	BrandID    uint64    `gorm:"column:brand_id"`
	SortWeight int32     `gorm:"column:sort_weight"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (seriesRow) TableName() string { return "catalog_series" }

type optionRow struct {
	ID        uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	ProductID uint64 `gorm:"column:product_id"`
	Name      string `gorm:"column:name"`
	SortOrder int32  `gorm:"column:sort_order"`
}

func (optionRow) TableName() string { return "product_options" }

type valueRow struct {
	ID        uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	OptionID  uint64 `gorm:"column:option_id"`
	Value     string `gorm:"column:value"`
	SortOrder int32  `gorm:"column:sort_order"`
}

func (valueRow) TableName() string { return "option_values" }

type skuRow struct {
	ID                   uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	ProductID            uint64    `gorm:"column:product_id"`
	SkuCode              string    `gorm:"column:sku_code"`
	Title                string    `gorm:"column:title"`
	Status               uint8     `gorm:"column:status"`
	FulfillmentMode      uint8     `gorm:"column:fulfillment_mode"`
	SaleAmountMinor      int64     `gorm:"column:sale_amount_minor"`
	ReferenceAmountMinor int64     `gorm:"column:reference_amount_minor"`
	Currency             string    `gorm:"column:currency"`
	ExternalCode         string    `gorm:"column:external_code"`
	Version              uint64    `gorm:"column:version"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

func (skuRow) TableName() string { return "skus" }

type selectionRow struct {
	SkuID         uint64 `gorm:"column:sku_id;primaryKey"`
	OptionID      uint64 `gorm:"column:option_id;primaryKey"`
	OptionValueID uint64 `gorm:"column:option_value_id"`
}

func (selectionRow) TableName() string { return "sku_selections" }

type draftRow struct {
	ProductID     uint64    `gorm:"column:product_id;primaryKey"`
	BaseVersion   uint64    `gorm:"column:base_version"`
	TargetVersion uint64    `gorm:"column:target_version"`
	DraftJSON     string    `gorm:"column:draft_json"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (draftRow) TableName() string { return "product_revision_drafts" }

type snapshotRow struct {
	ProductID    uint64    `gorm:"column:product_id;primaryKey"`
	Version      uint64    `gorm:"column:version;primaryKey"`
	SnapshotJSON string    `gorm:"column:snapshot_json"`
	PublishedAt  time.Time `gorm:"column:published_at"`
}

func (snapshotRow) TableName() string { return "product_published_snapshots" }

// Repo 商品目录数据访问。
type Repo struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// ── 创建 ──

// Save 创建商品（DRAFT + version 1）：head/选项/值/SKU/选择全量入库（单事务），
// 返回完整商品（真实 ID）。校验已由 core.BuildProduct 完成。
func (r *Repo) Save(ctx context.Context, draft *core.DraftProduct) (*catalogpb.Product, error) {
	var out *catalogpb.Product
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head := productRow{ProductCode: draft.Code, Title: draft.Title, Status: StatusDraft, Version: 1}
		if err := tx.Create(&head).Error; err != nil {
			return err
		}
		optionIDs := make([]uint64, len(draft.Options))
		valueIDs := make([][]uint64, len(draft.Options))
		for oi, o := range draft.Options {
			opt := optionRow{ProductID: head.ID, Name: o.Name, SortOrder: o.SortOrder}
			if err := tx.Create(&opt).Error; err != nil {
				return err
			}
			optionIDs[oi] = opt.ID
			valueIDs[oi] = make([]uint64, len(o.Values))
			for vi, v := range o.Values {
				val := valueRow{OptionID: opt.ID, Value: v.Value, SortOrder: v.SortOrder}
				if err := tx.Create(&val).Error; err != nil {
					return err
				}
				valueIDs[oi][vi] = val.ID
			}
		}
		for _, s := range draft.SKUs {
			sku := skuRow{ProductID: head.ID, SkuCode: s.Code, Title: s.Title, Status: StatusDraft,
				FulfillmentMode: uint8(s.Fulfillment), SaleAmountMinor: s.SalePrice.AmountMinor,
				ReferenceAmountMinor: s.ReferencePrice.AmountMinor, Currency: s.SalePrice.Currency,
				ExternalCode: s.ExternalCode, Version: 1}
			if err := tx.Create(&sku).Error; err != nil {
				return err
			}
			for _, sel := range s.Selections {
				// 下标 → 真实 ID（选项/值刚入库，ID 已回填）
				row := selectionRow{SkuID: sku.ID, OptionID: optionIDs[sel.OptionIndex],
					OptionValueID: valueIDs[sel.OptionIndex][sel.ValueIndex]}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			}
		}
		var err error
		out, err = findLive(ctx, tx, head.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ── 读取 ──

// GetSKU 按 SKU ID 读取（play 发布实物奖校验用）；未找到 → ErrNotFound。
func (r *Repo) GetSKU(ctx context.Context, id uint64) (*catalogpb.SKU, error) {
	var row skuRow
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var selections []selectionRow
	if err := r.db.WithContext(ctx).Where("sku_id = ?", id).Order("option_id").Find(&selections).Error; err != nil {
		return nil, err
	}
	sku := &catalogpb.SKU{
		SkuId:           row.ID,
		ProductId:       row.ProductID,
		SkuCode:         row.SkuCode,
		Title:           row.Title,
		Status:          catalogpb.ProductStatus(row.Status),
		FulfillmentMode: catalogpb.FulfillmentMode(row.FulfillmentMode),
		SalePrice:       &catalogpb.Money{AmountMinor: row.SaleAmountMinor, Currency: row.Currency},
		ReferencePrice:  &catalogpb.Money{AmountMinor: row.ReferenceAmountMinor, Currency: row.Currency},
		ExternalCode:    row.ExternalCode,
	}
	for _, sel := range selections {
		sku.Selections = append(sku.Selections, &catalogpb.SKUSelection{OptionId: sel.OptionID, OptionValueId: sel.OptionValueID})
	}
	return sku, nil
}

// Find 查看商品：head ACTIVE → 读已发布快照（head.Version）；否则拼 live 表。
func (r *Repo) Find(ctx context.Context, id uint64) (*catalogpb.Product, error) {
	var head productRow
	if err := r.db.WithContext(ctx).First(&head, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if head.Status == StatusActive {
		return r.FindPublished(ctx, id, head.Version)
	}
	return findLive(ctx, r.db, id)
}

// List 商品列表（id 游标升序，取 size 行）。
func (r *Repo) List(ctx context.Context, afterID uint64, size uint32) ([]*catalogpb.ProductSummary, error) {
	var rows []productRow
	if err := r.db.WithContext(ctx).Where("id > ?", afterID).Order("id").Limit(int(size)).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*catalogpb.ProductSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, &catalogpb.ProductSummary{ProductId: row.ID, ProductCode: row.ProductCode,
			Title: row.Title, Status: catalogpb.ProductStatus(row.Status), Version: row.Version})
	}
	return out, nil
}

// FindPublished 读取已发布快照（不可变；任意历史版本）。
func (r *Repo) FindPublished(ctx context.Context, id, version uint64) (*catalogpb.Product, error) {
	var row snapshotRow
	if err := r.db.WithContext(ctx).First(&row, "product_id = ? AND version = ?", id, version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return unmarshalSnapshot(row.SnapshotJSON)
}

// findLive 拼装 live 表（head 非 ACTIVE 时使用；行序照抄 server_v2 查询序）。
func findLive(ctx context.Context, q *gorm.DB, id uint64) (*catalogpb.Product, error) {
	var head productRow
	if err := q.WithContext(ctx).First(&head, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var options []optionRow
	if err := q.WithContext(ctx).Where("product_id = ?", id).Order("sort_order,id").Find(&options).Error; err != nil {
		return nil, err
	}
	var values []valueRow
	if err := q.WithContext(ctx).Table("option_values v").
		Joins("JOIN product_options o ON o.id = v.option_id").
		Where("o.product_id = ?", id).
		Order("o.sort_order,v.sort_order,v.id").
		Find(&values).Error; err != nil {
		return nil, err
	}
	var skus []skuRow
	if err := q.WithContext(ctx).Where("product_id = ?", id).Order("id").Find(&skus).Error; err != nil {
		return nil, err
	}
	var selections []selectionRow
	if err := q.WithContext(ctx).Table("sku_selections s").
		Joins("JOIN skus k ON k.id = s.sku_id").
		Where("k.product_id = ?", id).
		Order("s.sku_id,s.option_id").
		Find(&selections).Error; err != nil {
		return nil, err
	}
	p := &catalogpb.Product{ProductId: head.ID, ProductCode: head.ProductCode, Title: head.Title,
		Status: catalogpb.ProductStatus(head.Status), Version: head.Version}
	optionIndex := map[uint64]int{}
	for _, row := range options {
		optionIndex[row.ID] = len(p.Options)
		p.Options = append(p.Options, &catalogpb.ProductOption{OptionId: row.ID, Name: row.Name, SortOrder: row.SortOrder})
	}
	for _, row := range values {
		if idx, ok := optionIndex[row.OptionID]; ok {
			p.Options[idx].Values = append(p.Options[idx].Values,
				&catalogpb.OptionValue{OptionValueId: row.ID, OptionId: row.OptionID, Value: row.Value, SortOrder: row.SortOrder})
		}
	}
	skuIndex := map[uint64]int{}
	for _, row := range skus {
		skuIndex[row.ID] = len(p.Skus)
		p.Skus = append(p.Skus, &catalogpb.SKU{SkuId: row.ID, ProductId: row.ProductID, SkuCode: row.SkuCode,
			Title: row.Title, Status: catalogpb.ProductStatus(row.Status),
			FulfillmentMode: catalogpb.FulfillmentMode(row.FulfillmentMode),
			SalePrice:       &catalogpb.Money{AmountMinor: row.SaleAmountMinor, Currency: row.Currency},
			ReferencePrice:  &catalogpb.Money{AmountMinor: row.ReferenceAmountMinor, Currency: row.Currency},
			ExternalCode:    row.ExternalCode})
	}
	for _, row := range selections {
		if idx, ok := skuIndex[row.SkuID]; ok {
			p.Skus[idx].Selections = append(p.Skus[idx].Selections,
				&catalogpb.SKUSelection{OptionId: row.OptionID, OptionValueId: row.OptionValueID})
		}
	}
	return p, nil
}

// ── 首次发布 ──

// Publish 首次发布（DRAFT→ACTIVE）：锁 head → 幂等重放判定 → 激活 SKU（version+1）
// → 激活 head（version=version+1，快照版本随之前进）→ 插快照（单事务）。
func (r *Repo) Publish(ctx context.Context, id, expectedVersion uint64) (*catalogpb.Product, error) {
	var out *catalogpb.Product
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := lockHead(ctx, tx, id)
		if err != nil {
			return err
		}
		if head.Status == StatusActive && expectedVersion < math.MaxUint64 && head.Version == expectedVersion+1 {
			return errReplay // 已发布幂等重放：回滚后走快照读
		}
		if head.Status != StatusDraft || head.Version != expectedVersion {
			return ErrConflict
		}
		var count int64
		if err := tx.Model(&skuRow{}).Where("product_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrConflict
		}
		now := time.Now()
		res := tx.Model(&skuRow{}).Where("product_id = ? AND status = ?", id, StatusDraft).
			Updates(map[string]interface{}{"status": StatusActive, "version": gorm.Expr("version + 1"), "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != count {
			return ErrConflict
		}
		res = tx.Model(&productRow{}).Where("id = ? AND status = ? AND version = ?", id, StatusDraft, expectedVersion).
			Updates(map[string]interface{}{"status": StatusActive, "version": gorm.Expr("version + 1"), "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrConflict
		}
		snapshot, err := findLive(ctx, tx, id)
		if err != nil {
			return err
		}
		blob, err := protojson.Marshal(snapshot)
		if err != nil {
			return err
		}
		if err := tx.Create(&snapshotRow{ProductID: id, Version: snapshot.Version, SnapshotJSON: string(blob),
			PublishedAt: now}).Error; err != nil {
			return err
		}
		out = snapshot
		return nil
	})
	if errors.Is(err, errReplay) {
		return r.Find(ctx, id)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ── 修订 ──

// SaveRevision 保存修订草稿 blob：锁 head 校验 ACTIVE+版本+草稿形状 →
// 已有草稿同内容幂等返回/异内容冲突 → 插入草稿行（单事务）。
func (r *Repo) SaveRevision(ctx context.Context, id, baseVersion uint64, stored *core.StoredProduct) (*catalogpb.Product, error) {
	var out *catalogpb.Product
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := lockHead(ctx, tx, id)
		if err != nil {
			return err
		}
		if head.Status != StatusActive || head.Version != baseVersion ||
			stored.ID != id || stored.Version != baseVersion+1 {
			return ErrConflict
		}
		var existing draftRow
		err = tx.WithContext(ctx).First(&existing, "product_id = ?", id).Error
		if err == nil {
			var persisted core.StoredProduct
			if err := json.Unmarshal([]byte(existing.DraftJSON), &persisted); err != nil {
				return err
			}
			if existing.BaseVersion != baseVersion || existing.TargetVersion != stored.Version ||
				!core.EqualStored(&persisted, stored) {
				return ErrConflict
			}
			out = StoredToPb(stored)
			return nil // 同内容幂等返回
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		blob, err := json.Marshal(stored)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Create(&draftRow{ProductID: id, BaseVersion: baseVersion, TargetVersion: stored.Version,
			DraftJSON: string(blob), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			return err
		}
		out = StoredToPb(stored)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FindRevision 读取修订草稿（无草稿=商品不存在，照抄 server_v2）。
func (r *Repo) FindRevision(ctx context.Context, id uint64) (*catalogpb.Product, error) {
	var row draftRow
	if err := r.db.WithContext(ctx).First(&row, "product_id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var stored core.StoredProduct
	if err := json.Unmarshal([]byte(row.DraftJSON), &stored); err != nil {
		return nil, err
	}
	return StoredToPb(&stored), nil
}

// PublishRevision 发布修订：锁 head → 幂等重放判定（head.Version==expected+1）→
// 草稿/上版快照校验 → 插新增选项值 → head 前进 version=version+1 → 保留 SKU 更新为
// ACTIVE/新 SKU 插入/弃用 SKU 归档 → 插快照（target 版）→ 删草稿（单事务）。
func (r *Repo) PublishRevision(ctx context.Context, id, expectedVersion uint64) (*catalogpb.Product, error) {
	var out *catalogpb.Product
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		head, err := lockHead(ctx, tx, id)
		if err != nil {
			return err
		}
		if head.Status != StatusActive {
			return ErrConflict
		}
		if head.Version == expectedVersion+1 {
			return errReplay // 已发布幂等重放：回滚后读快照
		}
		if head.Version != expectedVersion {
			return ErrConflict
		}
		var row draftRow
		if err := tx.WithContext(ctx).First(&row, "product_id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrConflict
			}
			return err
		}
		if row.BaseVersion != expectedVersion || row.TargetVersion != expectedVersion+1 {
			return ErrConflict
		}
		var published core.StoredProduct
		if err := json.Unmarshal([]byte(row.DraftJSON), &published); err != nil {
			return err
		}
		if published.ID != id || published.Version != row.TargetVersion ||
			published.Status != catalogpb.ProductStatus_PRODUCT_STATUS_DRAFT {
			return ErrConflict
		}
		var prevRow snapshotRow
		if err := tx.WithContext(ctx).First(&prevRow, "product_id = ? AND version = ?", id, expectedVersion).Error; err != nil {
			return err // 上版快照缺失=内部不一致，原样上抛
		}
		previous, err := unmarshalSnapshot(prevRow.SnapshotJSON)
		if err != nil {
			return err
		}
		if len(previous.Options) != len(published.Options) {
			return ErrConflict
		}
		oldOptions := make(map[uint64]*catalogpb.ProductOption, len(previous.Options))
		for _, option := range previous.Options {
			oldOptions[option.OptionId] = option
		}
		now := time.Now()
		for i := range published.Options {
			option := &published.Options[i]
			old, ok := oldOptions[option.ID]
			if !ok || old.Name != option.Name {
				return ErrConflict
			}
			oldValues := make(map[uint64]bool, len(old.Values))
			for _, value := range old.Values {
				oldValues[value.OptionValueId] = true
			}
			for j := range option.Values {
				value := &option.Values[j]
				if oldValues[value.ID] {
					continue
				}
				row := valueRow{OptionID: option.ID, Value: value.Value, SortOrder: value.SortOrder}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
				value.ID = row.ID // 新值 ID 回填（供新 SKU 选择与快照）
			}
		}
		res := tx.Model(&productRow{}).
			Where("id = ? AND status = ? AND version = ?", id, StatusActive, expectedVersion).
			Updates(map[string]interface{}{"title": published.Title, "status": StatusActive,
				"version": gorm.Expr("version + 1"), "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrConflict
		}
		oldSKUs := make(map[uint64]*catalogpb.SKU, len(previous.Skus))
		for _, sku := range previous.Skus {
			oldSKUs[sku.SkuId] = sku
		}
		retained := make(map[uint64]bool, len(published.SKUs))
		for i := range published.SKUs {
			sku := &published.SKUs[i]
			if old, exists := oldSKUs[sku.ID]; exists {
				if sku.Code != old.SkuCode {
					return ErrConflict
				}
				res = tx.Model(&skuRow{}).
					Where("id = ? AND product_id = ? AND status = ?", sku.ID, id, StatusActive).
					Updates(map[string]interface{}{"title": sku.Title, "external_code": sku.ExternalCode,
						"fulfillment_mode": uint8(sku.Fulfillment), "sale_amount_minor": sku.SalePrice.AmountMinor,
						"reference_amount_minor": sku.ReferencePrice.AmountMinor, "currency": sku.SalePrice.Currency,
						"version": gorm.Expr("version + 1"), "updated_at": now})
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected != 1 {
					return ErrConflict
				}
			} else {
				row := skuRow{ProductID: id, SkuCode: sku.Code, Title: sku.Title, Status: StatusActive,
					FulfillmentMode: uint8(sku.Fulfillment), SaleAmountMinor: sku.SalePrice.AmountMinor,
					ReferenceAmountMinor: sku.ReferencePrice.AmountMinor, Currency: sku.SalePrice.Currency,
					ExternalCode: sku.ExternalCode, Version: 1}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
				sku.ID = row.ID
				for _, sel := range sku.Selections {
					valueID := sel.ValueID
					if valueID == 0 {
						option := findStoredOption(&published, sel.OptionID)
						if option == nil || int(sel.ValueIndex) >= len(option.Values) {
							return ErrConflict
						}
						valueID = option.Values[sel.ValueIndex].ID
					}
					if err := tx.Create(&selectionRow{SkuID: sku.ID, OptionID: sel.OptionID,
						OptionValueID: valueID}).Error; err != nil {
						return err
					}
				}
			}
			retained[sku.ID] = true
			sku.Status = catalogpb.ProductStatus_PRODUCT_STATUS_ACTIVE
		}
		for _, old := range previous.Skus {
			if retained[old.SkuId] {
				continue
			}
			res = tx.Model(&skuRow{}).
				Where("id = ? AND product_id = ? AND status = ?", old.SkuId, id, StatusActive).
				Updates(map[string]interface{}{"status": StatusArchived, "updated_at": now})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return ErrConflict
			}
		}
		published.Status = catalogpb.ProductStatus_PRODUCT_STATUS_ACTIVE
		pb := StoredToPb(&published)
		blob, err := protojson.Marshal(pb)
		if err != nil {
			return err
		}
		if err := tx.Create(&snapshotRow{ProductID: id, Version: published.Version, SnapshotJSON: string(blob),
			PublishedAt: now}).Error; err != nil {
			return err
		}
		res = tx.Where("product_id = ? AND base_version = ? AND target_version = ?",
			id, expectedVersion, published.Version).Delete(&draftRow{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrConflict
		}
		out = pb
		return nil
	})
	if errors.Is(err, errReplay) {
		return r.FindPublished(ctx, id, expectedVersion+1)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ── 内部工具 ──

// errReplay 内部信号：已发布幂等重放（事务回滚后走快照读）。
var errReplay = errors.New("replay published")

// lockHead 锁商品头行（FOR UPDATE，仅 id/status/version，照抄 server_v2 LockProductForPublish）。
func lockHead(ctx context.Context, tx *gorm.DB, id uint64) (*productRow, error) {
	var head productRow
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "status", "version").First(&head, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &head, err
}

// unmarshalSnapshot 快照 blob → pb.Product（快照以 protojson 存储）。
func unmarshalSnapshot(blob string) (*catalogpb.Product, error) {
	var p catalogpb.Product
	if err := protojson.Unmarshal([]byte(blob), &p); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return &p, nil
}

// StoredToPb 草稿持久化形状 → pb.Product（修订读接口响应；新实体 ID=0 占位，
// 发布时分配——快照内永远是真实 ID）。
func StoredToPb(stored *core.StoredProduct) *catalogpb.Product {
	p := &catalogpb.Product{ProductId: stored.ID, ProductCode: stored.Code, Title: stored.Title,
		Status: stored.Status, Version: stored.Version}
	for _, o := range stored.Options {
		op := &catalogpb.ProductOption{OptionId: o.ID, Name: o.Name, SortOrder: o.SortOrder}
		for _, v := range o.Values {
			op.Values = append(op.Values, &catalogpb.OptionValue{OptionValueId: v.ID, OptionId: o.ID,
				Value: v.Value, SortOrder: v.SortOrder})
		}
		p.Options = append(p.Options, op)
	}
	for _, s := range stored.SKUs {
		sku := &catalogpb.SKU{SkuId: s.ID, ProductId: stored.ID, SkuCode: s.Code, Title: s.Title,
			Status: s.Status, FulfillmentMode: s.Fulfillment,
			SalePrice:      &catalogpb.Money{AmountMinor: s.SalePrice.AmountMinor, Currency: s.SalePrice.Currency},
			ReferencePrice: &catalogpb.Money{AmountMinor: s.ReferencePrice.AmountMinor, Currency: s.ReferencePrice.Currency},
			ExternalCode:   s.ExternalCode}
		for _, sel := range s.Selections {
			sku.Selections = append(sku.Selections, &catalogpb.SKUSelection{OptionId: sel.OptionID, OptionValueId: sel.ValueID})
		}
		p.Skus = append(p.Skus, sku)
	}
	return p
}

// findStoredOption 按 ID 定位草稿选项。
func findStoredOption(p *core.StoredProduct, optionID uint64) *core.StoredOption {
	for i := range p.Options {
		if p.Options[i].ID == optionID {
			return &p.Options[i]
		}
	}
	return nil
}

// ── IP 列表 ──

// ListIPs 按名称模糊搜索 + 排序权重倒序分页。
func (r *Repo) ListIPs(ctx context.Context, keyword string, page, pageSize uint32) ([]*catalogpb.IP, uint64, error) {
	query := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&ipRow{})
		if keyword != "" {
			q = q.Where("name LIKE ?", "%"+keyword+"%")
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ipRow
	offset := int64(page-1) * int64(pageSize)
	if err := query().Order("sort_weight DESC, id DESC").Offset(int(offset)).Limit(int(pageSize)).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*catalogpb.IP, 0, len(rows))
	for _, row := range rows {
		out = append(out, ipToProto(row))
	}
	return out, uint64(total), nil
}

// CreateIP 新建 IP；名称唯一冲突返回 ErrIPExists。
func (r *Repo) CreateIP(ctx context.Context, name string, sortWeight int32) (*catalogpb.IP, error) {
	row := ipRow{Name: name, SortWeight: sortWeight}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrIPExists
		}
		return nil, err
	}
	return ipToProto(row), nil
}

// UpdateIP 编辑 IP；不存在返回 ErrNotFound，名称冲突返回 ErrIPExists。
func (r *Repo) UpdateIP(ctx context.Context, id uint64, name string, sortWeight int32) (*catalogpb.IP, error) {
	var row ipRow
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	row.Name = name
	row.SortWeight = sortWeight
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrIPExists
		}
		return nil, err
	}
	return ipToProto(row), nil
}

func ipToProto(row ipRow) *catalogpb.IP {
	return &catalogpb.IP{IpId: row.ID, Name: row.Name, SortWeight: row.SortWeight}
}

// ── 品牌列表 ──

// ListBrands 按名称模糊搜索 + ID 倒序分页。
func (r *Repo) ListBrands(ctx context.Context, keyword string, page, pageSize uint32) ([]*catalogpb.Brand, uint64, error) {
	query := func() *gorm.DB {
		q := r.db.WithContext(ctx).Model(&brandRow{})
		if keyword != "" {
			q = q.Where("name LIKE ?", "%"+keyword+"%")
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []brandRow
	offset := int64(page-1) * int64(pageSize)
	if err := query().Order("id DESC").Offset(int(offset)).Limit(int(pageSize)).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*catalogpb.Brand, 0, len(rows))
	for _, row := range rows {
		out = append(out, brandToProto(row))
	}
	return out, uint64(total), nil
}

// CreateBrand 新建品牌；名称唯一冲突返回 ErrBrandExists。
func (r *Repo) CreateBrand(ctx context.Context, name, logo string) (*catalogpb.Brand, error) {
	row := brandRow{Name: name, Logo: logo}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrBrandExists
		}
		return nil, err
	}
	return brandToProto(row), nil
}

// UpdateBrand 编辑品牌；不存在返回 ErrNotFound，名称冲突返回 ErrBrandExists。
func (r *Repo) UpdateBrand(ctx context.Context, id uint64, name, logo string) (*catalogpb.Brand, error) {
	var row brandRow
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	row.Name = name
	row.Logo = logo
	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrBrandExists
		}
		return nil, err
	}
	return brandToProto(row), nil
}

func brandToProto(row brandRow) *catalogpb.Brand {
	return &catalogpb.Brand{BrandId: row.ID, Name: row.Name, Logo: row.Logo}
}

// ── 系列列表 ──

type seriesListRow struct {
	ID         uint64 `gorm:"column:id"`
	Name       string `gorm:"column:name"`
	BrandID    uint64 `gorm:"column:brand_id"`
	BrandName  string `gorm:"column:brand_name"`
	SortWeight int32  `gorm:"column:sort_weight"`
}

// ListSeries 按系列名称模糊搜索 + 品牌筛选 + 排序权重倒序分页。
func (r *Repo) ListSeries(ctx context.Context, keyword string, brandID uint64, page, pageSize uint32) ([]*catalogpb.Series, uint64, error) {
	query := func() *gorm.DB {
		q := r.db.WithContext(ctx).Table("`catalog_series` AS s")
		if keyword != "" {
			q = q.Where("s.name LIKE ?", "%"+keyword+"%")
		}
		if brandID != 0 {
			q = q.Where("s.brand_id = ?", brandID)
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []seriesListRow
	offset := int64(page-1) * int64(pageSize)
	if err := query().
		Select("s.id, s.name, s.brand_id, b.name AS brand_name, s.sort_weight").
		Joins("JOIN `catalog_brands` AS b ON b.id = s.brand_id").
		Order("s.sort_weight DESC, s.id DESC").
		Offset(int(offset)).Limit(int(pageSize)).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*catalogpb.Series, 0, len(rows))
	for _, row := range rows {
		out = append(out, &catalogpb.Series{
			SeriesId: row.ID, Name: row.Name, BrandId: row.BrandID,
			BrandName: row.BrandName, SortWeight: row.SortWeight,
		})
	}
	return out, uint64(total), nil
}

// CreateSeries 新建系列；名称冲突返回 ErrSeriesExists，品牌不存在返回 ErrBrandNotFound。
func (r *Repo) CreateSeries(ctx context.Context, name string, brandID uint64, sortWeight int32) (*catalogpb.Series, error) {
	brand, err := r.findBrand(ctx, brandID)
	if err != nil {
		return nil, err
	}
	row := seriesRow{Name: name, BrandID: brandID, SortWeight: sortWeight}
	if err = r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrSeriesExists
		}
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			return nil, ErrBrandNotFound
		}
		return nil, err
	}
	return seriesToProto(row, brand.Name), nil
}

// UpdateSeries 编辑系列；不存在返回 ErrNotFound，名称冲突返回 ErrSeriesExists。
func (r *Repo) UpdateSeries(ctx context.Context, id uint64, name string, brandID uint64, sortWeight int32) (*catalogpb.Series, error) {
	var row seriesRow
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	brand, err := r.findBrand(ctx, brandID)
	if err != nil {
		return nil, err
	}
	row.Name = name
	row.BrandID = brandID
	row.SortWeight = sortWeight
	if err = r.db.WithContext(ctx).Save(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrSeriesExists
		}
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			return nil, ErrBrandNotFound
		}
		return nil, err
	}
	return seriesToProto(row, brand.Name), nil
}

func (r *Repo) findBrand(ctx context.Context, id uint64) (*brandRow, error) {
	var row brandRow
	if err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBrandNotFound
		}
		return nil, err
	}
	return &row, nil
}

func seriesToProto(row seriesRow, brandName string) *catalogpb.Series {
	return &catalogpb.Series{
		SeriesId: row.ID, Name: row.Name, BrandId: row.BrandID,
		BrandName: brandName, SortWeight: row.SortWeight,
	}
}
