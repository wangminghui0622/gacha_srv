// Package core 商品目录纯函数（M14c）：创建校验 + 变体身份映射。
// 校验规则与变体映射逐条移植 server_v2 catalog biz/catalog.go（buildProduct /
// SaveVariantRevision / sameSelections），纯函数可单测。
package core

import (
	"fmt"
	"strings"

	catalogpb "gacha_srv/app/catalog/catalog"
)

// Money 金额（最小单位 + 币种）
type Money struct {
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}

// DraftValue 草稿选项值（ID 由入库/发布时分配）
type DraftValue struct {
	Value     string
	SortOrder int32
}

// DraftOption 草稿选项
type DraftOption struct {
	Name      string
	SortOrder int32
	Values    []DraftValue
}

// DraftSelection 草稿 SKU 规格选择（构建阶段按下标）
type DraftSelection struct {
	OptionIndex int32 // 选项下标（指向 DraftProduct.Options）
	ValueIndex  int32 // 值下标（指向该选项的 Values）
}

// DraftSKU 草稿 SKU
type DraftSKU struct {
	Code          string
	Title         string
	ExternalCode  string
	Fulfillment   catalogpb.FulfillmentMode
	SalePrice     Money
	ReferencePrice Money
	Selections    []DraftSelection
}

// DraftProduct 创建/变体请求构建出的草稿（无 ID）
type DraftProduct struct {
	Code    string
	Title   string
	Options []DraftOption
	SKUs    []DraftSKU
}

// BuildProduct 校验创建请求并构建草稿（规则照抄 server_v2 biz.buildProduct）：
// code/title 非空；至少 1 SKU；选项名非空不重复、每选项 ≥1 值、值非空不重复；
// SKU 编码非空不重复；履约方式必填；sale 币种非空且 sale==reference 币种；
// 每个 SKU 为每个选项恰选一个合法值；同规格组合不得重复。
func BuildProduct(in *catalogpb.CreateProductRequest) (*DraftProduct, error) {
	if strings.TrimSpace(in.ProductCode) == "" || strings.TrimSpace(in.Title) == "" {
		return nil, fmt.Errorf("商品编码与标题必填")
	}
	if len(in.Skus) == 0 {
		return nil, fmt.Errorf("至少需要一个 SKU")
	}
	p := &DraftProduct{Code: in.ProductCode, Title: in.Title}
	optionNames := map[string]bool{}
	for _, o := range in.Options {
		name := strings.TrimSpace(o.Name)
		if name == "" || optionNames[name] {
			return nil, fmt.Errorf("选项名非法或重复 %q", name)
		}
		optionNames[name] = true
		op := DraftOption{Name: name, SortOrder: o.SortOrder}
		values := map[string]bool{}
		for _, v := range o.Values {
			value := strings.TrimSpace(v.Value)
			if value == "" || values[value] {
				return nil, fmt.Errorf("选项 %q 的值非法或重复 %q", name, value)
			}
			values[value] = true
			op.Values = append(op.Values, DraftValue{Value: value, SortOrder: v.SortOrder})
		}
		if len(op.Values) == 0 {
			return nil, fmt.Errorf("选项 %q 至少需要一个值", name)
		}
		p.Options = append(p.Options, op)
	}
	skuCodes, combos := map[string]bool{}, map[string]bool{}
	for _, s := range in.Skus {
		if s.SkuCode == "" || skuCodes[s.SkuCode] {
			return nil, fmt.Errorf("SKU 编码非法或重复 %q", s.SkuCode)
		}
		skuCodes[s.SkuCode] = true
		if s.FulfillmentMode == catalogpb.FulfillmentMode_FULFILLMENT_MODE_UNSPECIFIED {
			return nil, fmt.Errorf("SKU %q 履约方式必填", s.SkuCode)
		}
		if s.SalePrice.Currency == "" || s.SalePrice.Currency != s.ReferencePrice.Currency {
			return nil, fmt.Errorf("SKU %q 币种不一致", s.SkuCode)
		}
		if len(s.Selections) != len(p.Options) {
			return nil, fmt.Errorf("SKU %q 必须为每个选项选择一个值", s.SkuCode)
		}
		sku := DraftSKU{Code: s.SkuCode, Title: s.Title, ExternalCode: s.ExternalCode,
			Fulfillment: s.FulfillmentMode, SalePrice: Money{AmountMinor: s.SalePrice.AmountMinor, Currency: s.SalePrice.Currency},
			ReferencePrice: Money{AmountMinor: s.ReferencePrice.AmountMinor, Currency: s.ReferencePrice.Currency}}
		seen := map[int32]bool{}
		combo := ""
		for _, sel := range s.Selections {
			if sel.OptionIndex < 0 || sel.OptionIndex >= int32(len(p.Options)) || seen[sel.OptionIndex] {
				return nil, fmt.Errorf("SKU %q 选项选择非法", s.SkuCode)
			}
			op := p.Options[sel.OptionIndex]
			if sel.ValueIndex < 0 || sel.ValueIndex >= int32(len(op.Values)) {
				return nil, fmt.Errorf("SKU %q 值选择非法", s.SkuCode)
			}
			seen[sel.OptionIndex] = true
			sku.Selections = append(sku.Selections, DraftSelection{OptionIndex: sel.OptionIndex, ValueIndex: sel.ValueIndex})
			combo += fmt.Sprintf("%d:%d;", sel.OptionIndex, sel.ValueIndex)
		}
		if combos[combo] {
			return nil, fmt.Errorf("SKU 规格组合重复")
		}
		combos[combo] = true
		p.SKUs = append(p.SKUs, sku)
	}
	return p, nil
}

// StoredValue 持久化选项值（ID=0 表示新值，修订发布时插入分配）
type StoredValue struct {
	ID        uint64 `json:"id,omitempty"`
	Value     string `json:"value"`
	SortOrder int32  `json:"sortOrder"`
}

// StoredOption 持久化选项（ID 恒为现版本真实 ID）
type StoredOption struct {
	ID        uint64        `json:"id"`
	Name      string        `json:"name"`
	SortOrder int32         `json:"sortOrder"`
	Values    []StoredValue `json:"values"`
}

// StoredSelection 持久化规格选择：ValueID=0 时按 ValueIndex 定位新值
type StoredSelection struct {
	OptionID   uint64 `json:"optionId"`
	ValueID    uint64 `json:"valueId,omitempty"` // 0=新值
	ValueIndex int32  `json:"valueIndex"`        // 新值时：值在所属选项 Values 列表中的下标
}

// StoredSKU 持久化 SKU（ID=0 表示新 SKU，修订发布时插入分配）
type StoredSKU struct {
	ID             uint64                 `json:"id,omitempty"`
	Code           string                 `json:"code"`
	Title          string                 `json:"title"`
	ExternalCode   string                 `json:"externalCode"`
	Status         catalogpb.ProductStatus `json:"status"`
	Fulfillment    catalogpb.FulfillmentMode `json:"fulfillmentMode"`
	SalePrice      Money                  `json:"salePrice"`
	ReferencePrice Money                  `json:"referencePrice"`
	Selections     []StoredSelection      `json:"selections"`
}

// StoredProduct 修订草稿持久化形状（draft_json blob；照抄 server_v2 draft blob 语义）
type StoredProduct struct {
	ID      uint64          `json:"id"`
	Code    string          `json:"code"`
	Title   string          `json:"title"`
	Version uint64          `json:"version"` // target version（= base+1）
	Status  catalogpb.ProductStatus `json:"status"`
	Options []StoredOption  `json:"options"`
	SKUs    []StoredSKU     `json:"skus"`
}

// MakeRevisionDraft 构建普通修订草稿（规则照抄 server_v2 biz.SaveRevision）：
// 现版本必须 ACTIVE 且 version==expected、修订 SKU 数与现 SKU 数一致；
// 每个修订 SKU 必须按 ID 命中现 SKU、不重复、title 非空、履约方式必填、
// 金额 ≥0、币种 3 字母且 sale==reference 币种。不符返回 ErrVariantConflict。
// 返回 StoredProduct（clone 现版本 + 修改 + version+1 + DRAFT）。
func MakeRevisionDraft(current *catalogpb.Product, expectedVersion uint64, title string, skuIn []*catalogpb.ReviseSKUInput) (*StoredProduct, error) {
	if current.Status != catalogpb.ProductStatus_PRODUCT_STATUS_ACTIVE || current.Version != expectedVersion ||
		len(current.Skus) != len(skuIn) {
		return nil, ErrVariantConflict
	}
	out := &StoredProduct{ID: current.ProductId, Code: current.ProductCode, Title: strings.TrimSpace(title),
		Version: current.Version + 1, Status: catalogpb.ProductStatus_PRODUCT_STATUS_DRAFT}
	for _, o := range current.Options {
		so := StoredOption{ID: o.OptionId, Name: o.Name, SortOrder: o.SortOrder}
		for _, v := range o.Values {
			so.Values = append(so.Values, StoredValue{ID: v.OptionValueId, Value: v.Value, SortOrder: v.SortOrder})
		}
		out.Options = append(out.Options, so)
	}
	index := make(map[uint64]int, len(current.Skus))
	skus := make([]StoredSKU, len(current.Skus))
	for i := range current.Skus {
		old := current.Skus[i]
		index[old.SkuId] = i
		// 未修改的 SKU 保持原字段（clone 语义）
		skus[i] = StoredSKU{ID: old.SkuId, Code: old.SkuCode, Title: old.Title,
			ExternalCode: old.ExternalCode, Status: catalogpb.ProductStatus_PRODUCT_STATUS_DRAFT,
			Fulfillment: old.FulfillmentMode,
			SalePrice:      Money{AmountMinor: old.SalePrice.AmountMinor, Currency: old.SalePrice.Currency},
			ReferencePrice: Money{AmountMinor: old.ReferencePrice.AmountMinor, Currency: old.ReferencePrice.Currency}}
		for _, sel := range old.Selections {
			skus[i].Selections = append(skus[i].Selections, StoredSelection{OptionID: sel.OptionId, ValueID: sel.OptionValueId})
		}
	}
	seen := map[uint64]bool{}
	for _, changed := range skuIn {
		i, ok := index[changed.SkuId]
		if !ok || seen[changed.SkuId] || strings.TrimSpace(changed.Title) == "" ||
			changed.FulfillmentMode == catalogpb.FulfillmentMode_FULFILLMENT_MODE_UNSPECIFIED ||
			changed.SalePrice.AmountMinor < 0 || changed.ReferencePrice.AmountMinor < 0 ||
			len(changed.SalePrice.Currency) != 3 || changed.SalePrice.Currency != changed.ReferencePrice.Currency {
			return nil, ErrVariantConflict
		}
		seen[changed.SkuId] = true
		skus[i].Title = strings.TrimSpace(changed.Title)
		skus[i].ExternalCode = changed.ExternalCode
		skus[i].Fulfillment = changed.FulfillmentMode
		skus[i].SalePrice = Money{AmountMinor: changed.SalePrice.AmountMinor, Currency: changed.SalePrice.Currency}
		skus[i].ReferencePrice = Money{AmountMinor: changed.ReferencePrice.AmountMinor, Currency: changed.ReferencePrice.Currency}
	}
	out.SKUs = skus
	return out, nil
}

// ErrVariantConflict 变体身份映射冲突（选项名集合不符/稳定 SKU 换组合等）。
var ErrVariantConflict = fmt.Errorf("variant identity conflict")

// MakeVariantRevision 把新构建的草稿映射到现版本身份空间（规则照抄 server_v2
// biz.SaveVariantRevision）：
//   选项数必须相等且选项名集合一致（选项 ID 复用旧 ID）；
//   同值名复用旧值 ID，新值 ID=0；同 SKU code 复用旧 SKU ID 且组合不可变，新 SKU ID=0。
// 返回 StoredProduct（Status=DRAFT、Version=current+1），身份冲突返回 ErrVariantConflict。
func MakeVariantRevision(draft *DraftProduct, current *catalogpb.Product) (*StoredProduct, error) {
	if len(draft.Options) != len(current.Options) {
		return nil, ErrVariantConflict // 现有选项维度定义 SKU 身份，维度数不可变
	}
	oldOptions := make(map[string]*catalogpb.ProductOption, len(current.Options))
	for i := range current.Options {
		oldOptions[current.Options[i].Name] = current.Options[i]
	}
	out := &StoredProduct{ID: current.ProductId, Code: draft.Code, Title: draft.Title,
		Version: current.Version + 1, Status: catalogpb.ProductStatus_PRODUCT_STATUS_DRAFT}
	for i := range draft.Options {
		dopt := &draft.Options[i]
		old, ok := oldOptions[dopt.Name]
		if !ok {
			return nil, ErrVariantConflict
		}
		so := StoredOption{ID: old.OptionId, Name: dopt.Name, SortOrder: dopt.SortOrder}
		oldValues := make(map[string]uint64, len(old.Values))
		for _, v := range old.Values {
			oldValues[v.Value] = v.OptionValueId
		}
		for j := range dopt.Values {
			dv := &dopt.Values[j]
			sv := StoredValue{Value: dv.Value, SortOrder: dv.SortOrder}
			if oldID := oldValues[dv.Value]; oldID != 0 {
				sv.ID = oldID
			}
			so.Values = append(so.Values, sv)
		}
		out.Options = append(out.Options, so)
	}
	oldSKUs := make(map[string]*catalogpb.SKU, len(current.Skus))
	for i := range current.Skus {
		oldSKUs[current.Skus[i].SkuCode] = current.Skus[i]
	}
	for i := range draft.SKUs {
		dsku := &draft.SKUs[i]
		sku := StoredSKU{Code: dsku.Code, Title: dsku.Title, ExternalCode: dsku.ExternalCode,
			Status: catalogpb.ProductStatus_PRODUCT_STATUS_DRAFT, Fulfillment: dsku.Fulfillment,
			SalePrice: dsku.SalePrice, ReferencePrice: dsku.ReferencePrice}
		// 规格选择映射：选项下标 → 真实选项 ID；值下标 → 旧值 ID 或 0+ValueIndex
		byOption := map[uint64]uint64{}
		for _, sel := range dsku.Selections {
			so := out.Options[sel.OptionIndex]
			sv := so.Values[sel.ValueIndex]
			ss := StoredSelection{OptionID: so.ID, ValueID: sv.ID, ValueIndex: sel.ValueIndex}
			sku.Selections = append(sku.Selections, ss)
			byOption[so.ID] = sv.ID
		}
		if old, exists := oldSKUs[dsku.Code]; exists {
			if !sameSelections(old.Selections, byOption) {
				return nil, ErrVariantConflict // 稳定 SKU 不允许更换规格组合
			}
			sku.ID = old.SkuId
		}
		out.SKUs = append(out.SKUs, sku)
	}
	return out, nil
}

// sameSelections 现版本 SKU 组合与草稿组合是否一致（按选项 ID 比对值 ID）。
func sameSelections(old []*catalogpb.SKUSelection, byOption map[uint64]uint64) bool {
	if len(old) != len(byOption) {
		return false
	}
	for _, s := range old {
		if byOption[s.OptionId] != s.OptionValueId {
			return false
		}
	}
	return true
}

// CloneStored 深拷贝草稿（与已存草稿做内容比较用）。
func CloneStored(p *StoredProduct) *StoredProduct {
	copy := *p
	copy.Options = append([]StoredOption(nil), p.Options...)
	for i := range copy.Options {
		copy.Options[i].Values = append([]StoredValue(nil), p.Options[i].Values...)
	}
	copy.SKUs = append([]StoredSKU(nil), p.SKUs...)
	for i := range copy.SKUs {
		copy.SKUs[i].Selections = append([]StoredSelection(nil), p.SKUs[i].Selections...)
	}
	return &copy
}

// EqualStored 草稿内容相等（用于修订幂等判定）。
func EqualStored(a, b *StoredProduct) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.ID != b.ID || a.Code != b.Code || a.Title != b.Title || a.Version != b.Version || a.Status != b.Status ||
		len(a.Options) != len(b.Options) || len(a.SKUs) != len(b.SKUs) {
		return false
	}
	for i := range a.Options {
		ao, bo := &a.Options[i], &b.Options[i]
		if ao.ID != bo.ID || ao.Name != bo.Name || ao.SortOrder != bo.SortOrder || len(ao.Values) != len(bo.Values) {
			return false
		}
		for j := range ao.Values {
			av, bv := &ao.Values[j], &bo.Values[j]
			if av.ID != bv.ID || av.Value != bv.Value || av.SortOrder != bv.SortOrder {
				return false
			}
		}
	}
	for i := range a.SKUs {
		as, bs := &a.SKUs[i], &b.SKUs[i]
		if as.ID != bs.ID || as.Code != bs.Code || as.Title != bs.Title || as.ExternalCode != bs.ExternalCode ||
			as.Status != bs.Status || as.Fulfillment != bs.Fulfillment || as.SalePrice != bs.SalePrice ||
			as.ReferencePrice != bs.ReferencePrice || len(as.Selections) != len(bs.Selections) {
			return false
		}
		for j := range as.Selections {
			if as.Selections[j] != bs.Selections[j] {
				return false
			}
		}
	}
	return true
}
