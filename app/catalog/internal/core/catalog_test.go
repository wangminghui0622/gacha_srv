package core

import (
	"errors"
	"testing"

	catalogpb "gacha_srv/app/catalog/catalog"
)

func money(amount int64, currency string) *catalogpb.Money {
	return &catalogpb.Money{AmountMinor: amount, Currency: currency}
}

// 标准创建请求：2 选项（颜色×2 值、尺寸×2 值）+ 4 SKU 全组合
func stdCreateRequest() *catalogpb.CreateProductRequest {
	return &catalogpb.CreateProductRequest{
		ProductCode: "SPU-001",
		Title:       "扭蛋皮肤包",
		Options: []*catalogpb.CreateProductOptionInput{
			{Name: "颜色", SortOrder: 1, Values: []*catalogpb.CreateOptionValueInput{
				{Value: "红", SortOrder: 1}, {Value: "蓝", SortOrder: 2}}},
			{Name: "尺寸", SortOrder: 2, Values: []*catalogpb.CreateOptionValueInput{
				{Value: "大", SortOrder: 1}, {Value: "小", SortOrder: 2}}},
		},
		Skus: []*catalogpb.CreateSKUInput{
			{SkuCode: "S-红-大", Title: "红大", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
				SalePrice: money(100, "CNY"), ReferencePrice: money(200, "CNY"),
				Selections: []*catalogpb.CreateSKUSelectionInput{{OptionIndex: 0, ValueIndex: 0}, {OptionIndex: 1, ValueIndex: 0}}},
			{SkuCode: "S-红-小", Title: "红小", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
				SalePrice: money(100, "CNY"), ReferencePrice: money(200, "CNY"),
				Selections: []*catalogpb.CreateSKUSelectionInput{{OptionIndex: 0, ValueIndex: 0}, {OptionIndex: 1, ValueIndex: 1}}},
			{SkuCode: "S-蓝-大", Title: "蓝大", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
				SalePrice: money(100, "CNY"), ReferencePrice: money(200, "CNY"),
				Selections: []*catalogpb.CreateSKUSelectionInput{{OptionIndex: 0, ValueIndex: 1}, {OptionIndex: 1, ValueIndex: 0}}},
			{SkuCode: "S-蓝-小", Title: "蓝小", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
				SalePrice: money(100, "CNY"), ReferencePrice: money(200, "CNY"),
				Selections: []*catalogpb.CreateSKUSelectionInput{{OptionIndex: 0, ValueIndex: 1}, {OptionIndex: 1, ValueIndex: 1}}},
		},
	}
}

// 标准现版本（已发布 v2，ID 全部真实）
func stdCurrentProduct() *catalogpb.Product {
	return &catalogpb.Product{
		ProductId: 10, ProductCode: "SPU-001", Title: "扭蛋皮肤包",
		Status: catalogpb.ProductStatus_PRODUCT_STATUS_ACTIVE, Version: 2,
		Options: []*catalogpb.ProductOption{
			{OptionId: 101, Name: "颜色", SortOrder: 1, Values: []*catalogpb.OptionValue{
				{OptionValueId: 1001, OptionId: 101, Value: "红", SortOrder: 1},
				{OptionValueId: 1002, OptionId: 101, Value: "蓝", SortOrder: 2}}},
			{OptionId: 102, Name: "尺寸", SortOrder: 2, Values: []*catalogpb.OptionValue{
				{OptionValueId: 2001, OptionId: 102, Value: "大", SortOrder: 1},
				{OptionValueId: 2002, OptionId: 102, Value: "小", SortOrder: 2}}},
		},
		Skus: []*catalogpb.SKU{
			{SkuId: 301, ProductId: 10, SkuCode: "S-红-大", Title: "红大", Status: catalogpb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL, SalePrice: money(100, "CNY"),
				ReferencePrice: money(200, "CNY"),
				Selections:     []*catalogpb.SKUSelection{{OptionId: 101, OptionValueId: 1001}, {OptionId: 102, OptionValueId: 2001}}},
			{SkuId: 302, ProductId: 10, SkuCode: "S-红-小", Title: "红小", Status: catalogpb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL, SalePrice: money(100, "CNY"),
				ReferencePrice: money(200, "CNY"),
				Selections:     []*catalogpb.SKUSelection{{OptionId: 101, OptionValueId: 1001}, {OptionId: 102, OptionValueId: 2002}}},
			{SkuId: 303, ProductId: 10, SkuCode: "S-蓝-大", Title: "蓝大", Status: catalogpb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL, SalePrice: money(100, "CNY"),
				ReferencePrice: money(200, "CNY"),
				Selections:     []*catalogpb.SKUSelection{{OptionId: 101, OptionValueId: 1002}, {OptionId: 102, OptionValueId: 2001}}},
			{SkuId: 304, ProductId: 10, SkuCode: "S-蓝-小", Title: "蓝小", Status: catalogpb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL, SalePrice: money(100, "CNY"),
				ReferencePrice: money(200, "CNY"),
				Selections:     []*catalogpb.SKUSelection{{OptionId: 101, OptionValueId: 1002}, {OptionId: 102, OptionValueId: 2002}}},
		},
	}
}

func TestBuildProductOK(t *testing.T) {
	draft, err := BuildProduct(stdCreateRequest())
	if err != nil {
		t.Fatalf("BuildProduct: %v", err)
	}
	if len(draft.Options) != 2 || len(draft.SKUs) != 4 {
		t.Fatalf("结构不符: options=%d skus=%d", len(draft.Options), len(draft.SKUs))
	}
}

func TestBuildProductValidation(t *testing.T) {
	dup := func() *catalogpb.CreateProductRequest {
		req := stdCreateRequest()
		req.Options = append([]*catalogpb.CreateProductOptionInput(nil), stdCreateRequest().Options...)
		req.Skus = append([]*catalogpb.CreateSKUInput(nil), stdCreateRequest().Skus...)
		return req
	}
	cases := []struct {
		name string
		mut  func(*catalogpb.CreateProductRequest)
	}{
		{"空编码", func(r *catalogpb.CreateProductRequest) { r.ProductCode = "  " }},
		{"空标题", func(r *catalogpb.CreateProductRequest) { r.Title = "" }},
		{"无SKU", func(r *catalogpb.CreateProductRequest) { r.Skus = nil }},
		{"选项名为空", func(r *catalogpb.CreateProductRequest) { r.Options[0].Name = " " }},
		{"选项名重复", func(r *catalogpb.CreateProductRequest) { r.Options[1].Name = "颜色" }},
		{"选项无值", func(r *catalogpb.CreateProductRequest) { r.Options[0].Values = nil }},
		{"值重复", func(r *catalogpb.CreateProductRequest) { r.Options[0].Values[1].Value = "红" }},
		{"值非法", func(r *catalogpb.CreateProductRequest) { r.Options[0].Values[0].Value = " " }},
		{"SKU编码重复", func(r *catalogpb.CreateProductRequest) { r.Skus[1].SkuCode = "S-红-大" }},
		{"履约方式未指定", func(r *catalogpb.CreateProductRequest) { r.Skus[0].FulfillmentMode = catalogpb.FulfillmentMode_FULFILLMENT_MODE_UNSPECIFIED }},
		{"币种不一致", func(r *catalogpb.CreateProductRequest) { r.Skus[0].ReferencePrice.Currency = "USD" }},
		{"选择数不足", func(r *catalogpb.CreateProductRequest) { r.Skus[0].Selections = r.Skus[0].Selections[:1] }},
		{"选项下标越界", func(r *catalogpb.CreateProductRequest) { r.Skus[0].Selections[0].OptionIndex = 2 }},
		{"值下标越界", func(r *catalogpb.CreateProductRequest) { r.Skus[0].Selections[1].ValueIndex = 5 }},
		{"组合重复", func(r *catalogpb.CreateProductRequest) {
			r.Skus[1].Selections = append([]*catalogpb.CreateSKUSelectionInput(nil), r.Skus[0].Selections...)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := dup()
			c.mut(req)
			if _, err := BuildProduct(req); err == nil {
				t.Fatalf("期望校验失败")
			}
		})
	}
}

func TestMakeVariantRevision(t *testing.T) {
	req := stdCreateRequest()
	// 新增值「绿」（颜色第 3 值）+ 新 SKU「S-绿-大」；其余 SKU 保持
	req.Options[0].Values = append(req.Options[0].Values, &catalogpb.CreateOptionValueInput{Value: "绿", SortOrder: 3})
	req.Skus = append(req.Skus, &catalogpb.CreateSKUInput{
		SkuCode: "S-绿-大", Title: "绿大", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
		SalePrice: money(300, "CNY"), ReferencePrice: money(400, "CNY"),
		Selections: []*catalogpb.CreateSKUSelectionInput{{OptionIndex: 0, ValueIndex: 2}, {OptionIndex: 1, ValueIndex: 0}},
	})
	draft, err := BuildProduct(req)
	if err != nil {
		t.Fatalf("BuildProduct: %v", err)
	}
	stored, err := MakeVariantRevision(draft, stdCurrentProduct())
	if err != nil {
		t.Fatalf("MakeVariantRevision: %v", err)
	}
	if stored.ID != 10 || stored.Version != 3 || stored.Status != catalogpb.ProductStatus_PRODUCT_STATUS_DRAFT {
		t.Fatalf("草稿头不符: %+v", stored)
	}
	// 选项 ID 复用旧 ID；新值 ID=0
	if stored.Options[0].ID != 101 || stored.Options[1].ID != 102 {
		t.Fatalf("选项 ID 未复用: %+v", stored.Options)
	}
	if stored.Options[0].Values[0].ID != 1001 || stored.Options[0].Values[2].ID != 0 {
		t.Fatalf("值 ID 映射不符: %+v", stored.Options[0].Values)
	}
	// 稳定 SKU 复用 ID，新 SKU ID=0，新 SKU 选择指向新值（ValueID=0+ValueIndex=2）
	idByCode := map[string]uint64{}
	for _, sku := range stored.SKUs {
		idByCode[sku.Code] = sku.ID
	}
	if idByCode["S-红-大"] != 301 || idByCode["S-绿-大"] != 0 {
		t.Fatalf("SKU ID 映射不符: %+v", idByCode)
	}
	var green *StoredSKU
	for i := range stored.SKUs {
		if stored.SKUs[i].Code == "S-绿-大" {
			green = &stored.SKUs[i]
		}
	}
	if green == nil || len(green.Selections) != 2 ||
		green.Selections[0].OptionID != 101 || green.Selections[0].ValueID != 0 || green.Selections[0].ValueIndex != 2 ||
		green.Selections[1].OptionID != 102 || green.Selections[1].ValueID != 2001 {
		t.Fatalf("新 SKU 选择映射不符: %+v", green)
	}
}

func TestMakeVariantRevisionConflicts(t *testing.T) {
	// 选项维度数不可变
	req := stdCreateRequest()
	req.Options = req.Options[:1]
	req.Skus = req.Skus[:1]
	req.Skus[0].Selections = req.Skus[0].Selections[:1]
	draft, err := BuildProduct(req)
	if err != nil {
		t.Fatalf("BuildProduct: %v", err)
	}
	if _, err := MakeVariantRevision(draft, stdCurrentProduct()); !errors.Is(err, ErrVariantConflict) {
		t.Fatalf("期望维度冲突, got %v", err)
	}

	// 选项名集合必须一致
	req = stdCreateRequest()
	req.Options[0].Name = "材质"
	draft, _ = BuildProduct(req)
	if _, err := MakeVariantRevision(draft, stdCurrentProduct()); !errors.Is(err, ErrVariantConflict) {
		t.Fatalf("期望选项名冲突, got %v", err)
	}

	// 稳定 SKU 换组合 → 冲突
	req = stdCreateRequest()
	// S-红-大 改为 蓝+大（截取前 2 个 SKU，避免构建期与 S-蓝-大 撞组合）
	req.Skus = req.Skus[:2]
	req.Skus[0].Selections[0].ValueIndex = 1
	draft, _ = BuildProduct(req)
	if _, err := MakeVariantRevision(draft, stdCurrentProduct()); !errors.Is(err, ErrVariantConflict) {
		t.Fatalf("期望组合冲突, got %v", err)
	}
}

func TestMakeRevisionDraft(t *testing.T) {
	current := stdCurrentProduct()
	in := []*catalogpb.ReviseSKUInput{
		{SkuId: 301, Title: "红大·新", ExternalCode: "EXT-1",
			FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(150, "CNY"), ReferencePrice: money(250, "CNY")},
		{SkuId: 302, Title: "红小·新", ExternalCode: "EXT-2",
			FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(150, "CNY"), ReferencePrice: money(250, "CNY")},
		{SkuId: 303, Title: "蓝大·新", ExternalCode: "EXT-3",
			FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(150, "CNY"), ReferencePrice: money(250, "CNY")},
		{SkuId: 304, Title: "蓝小·新", ExternalCode: "EXT-4",
			FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(150, "CNY"), ReferencePrice: money(250, "CNY")},
	}
	draft, err := MakeRevisionDraft(current, 2, "扭蛋皮肤包·改", in)
	if err != nil {
		t.Fatalf("MakeRevisionDraft: %v", err)
	}
	if draft.Version != 3 || draft.Status != catalogpb.ProductStatus_PRODUCT_STATUS_DRAFT ||
		draft.Title != "扭蛋皮肤包·改" || len(draft.SKUs) != 4 {
		t.Fatalf("草稿不符: %+v", draft)
	}
	if draft.SKUs[0].SalePrice.AmountMinor != 150 || draft.SKUs[0].Status != catalogpb.ProductStatus_PRODUCT_STATUS_DRAFT {
		t.Fatalf("SKU 修改未生效: %+v", draft.SKUs[0])
	}
	if draft.SKUs[3].Code != "S-蓝-小" || draft.SKUs[3].SalePrice.AmountMinor != 150 {
		t.Fatalf("SKU 顺序应为现版本顺序: %+v", draft.SKUs[3])
	}
}

func TestMakeRevisionDraftConflicts(t *testing.T) {
	current := stdCurrentProduct()
	one := []*catalogpb.ReviseSKUInput{{SkuId: 301, Title: "t", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
		SalePrice: money(1, "CNY"), ReferencePrice: money(1, "CNY")}}
	cases := []struct {
		name    string
		version uint64
		skus    []*catalogpb.ReviseSKUInput
	}{
		{"版本不符", 1, one},
		{"SKU数不符", 2, one},
		{"SKU不存在", 2, []*catalogpb.ReviseSKUInput{{SkuId: 999, Title: "t", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(1, "CNY"), ReferencePrice: money(1, "CNY")}}},
		{"负数金额", 2, []*catalogpb.ReviseSKUInput{{SkuId: 301, Title: "t", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(-1, "CNY"), ReferencePrice: money(1, "CNY")}}},
		{"币种非3字母", 2, []*catalogpb.ReviseSKUInput{{SkuId: 301, Title: "t", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(1, "CN"), ReferencePrice: money(1, "CN")}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := MakeRevisionDraft(current, c.version, "title", c.skus); !errors.Is(err, ErrVariantConflict) {
				t.Fatalf("期望冲突, got %v", err)
			}
		})
	}
}

func TestEqualStored(t *testing.T) {
	current := stdCurrentProduct()
	in := []*catalogpb.ReviseSKUInput{
		{SkuId: 301, Title: "红大·新", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(150, "CNY"), ReferencePrice: money(250, "CNY")},
		{SkuId: 302, Title: "红小·新", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(150, "CNY"), ReferencePrice: money(250, "CNY")},
		{SkuId: 303, Title: "蓝大·新", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(150, "CNY"), ReferencePrice: money(250, "CNY")},
		{SkuId: 304, Title: "蓝小·新", FulfillmentMode: catalogpb.FulfillmentMode_FULFILLMENT_MODE_DIGITAL,
			SalePrice: money(150, "CNY"), ReferencePrice: money(250, "CNY")},
	}
	a, _ := MakeRevisionDraft(current, 2, "新标题", in)
	b, _ := MakeRevisionDraft(current, 2, "新标题", in)
	if !EqualStored(a, b) {
		t.Fatalf("同内容应相等")
	}
	a.Title = "不同"
	if EqualStored(a, b) {
		t.Fatalf("异内容不应相等")
	}
}
