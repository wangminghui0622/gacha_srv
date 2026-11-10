package core

import (
	"bytes"
	"testing"

	"gacha_srv/app/draw/draw"

	"google.golang.org/protobuf/proto"
)

// sampleDraft 构造合法草稿（不含密封字段）。
func sampleDraft() *draw.BoxDraft {
	return &draw.BoxDraft{
		BoxName:     "测试宝箱",
		Price:       100,
		Description: "测试",
		Entries: []*draw.PoolEntryDraft{
			{Name: "谢谢参与", Weight: 60, Quantity: -1},
			{Name: "金币x50", Weight: 30, Quantity: 100, IsPrize: true, DisplayValueFen: 100},
			{Name: "限定皮肤", Weight: 10, Quantity: 5, IsPrize: true, DisplayValueFen: 5000, RollingLimited: true},
		},
	}
}

func TestValidateDraft(t *testing.T) {
	if err := ValidateDraft(sampleDraft()); err != nil {
		t.Fatalf("合法草稿校验失败: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(b *draw.BoxDraft)
	}{
		{"空名称", func(b *draw.BoxDraft) { b.BoxName = "  " }},
		{"负价格", func(b *draw.BoxDraft) { b.Price = -1 }},
		{"无条目", func(b *draw.BoxDraft) { b.Entries = nil }},
		{"条目空名", func(b *draw.BoxDraft) { b.Entries[0].Name = "" }},
		{"负权重", func(b *draw.BoxDraft) { b.Entries[0].Weight = -1 }},
		{"库存0", func(b *draw.BoxDraft) { b.Entries[0].Quantity = 0 }},
		{"库存小于-1", func(b *draw.BoxDraft) { b.Entries[0].Quantity = -2 }},
		{"负展示价", func(b *draw.BoxDraft) { b.Entries[0].DisplayValueFen = -1 }},
		{"全零权重", func(b *draw.BoxDraft) {
			for _, e := range b.Entries {
				e.Weight = 0
			}
		}},
		{"滚动规则非法", func(b *draw.BoxDraft) {
			r := `{"high_value_threshold_fen":1000,"single_draw_excluded_percent":200}`
			b.RollingRulesJson = r
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := sampleDraft()
			tt.mutate(b)
			if err := ValidateDraft(b); err == nil {
				t.Fatalf("期望校验失败")
			}
		})
	}
}

func TestValidateDraftRollingRulesOK(t *testing.T) {
	b := sampleDraft()
	b.RollingRulesJson = `{"high_value_threshold_fen":3000,"single_draw_excluded_percent":0,"user_excluded_percent":0}`
	if err := ValidateDraft(b); err != nil {
		t.Fatalf("合法滚动规则校验失败: %v", err)
	}
}

func TestSealPoolDraft(t *testing.T) {
	b := sampleDraft()
	b.Status = draw.PoolStatus_POOL_STATUS_DRAFT

	if err := SealPoolDraft(b, 1); err != nil {
		t.Fatalf("密封失败: %v", err)
	}
	if b.Version != 1 || len(b.ConfigHash) != 32 {
		t.Fatalf("密封结果异常: version=%d hashLen=%d", b.Version, len(b.ConfigHash))
	}

	// 前置不满足：已密封再密封
	if err := SealPoolDraft(b, 2); err == nil {
		t.Fatal("重复密封应失败")
	}
	// version<=0 / 状态非法
	b2 := sampleDraft()
	b2.Status = draw.PoolStatus_POOL_STATUS_UNSPECIFIED
	if err := SealPoolDraft(b2, 1); err == nil {
		t.Fatal("未置 DRAFT 密封应失败")
	}
}

func TestSealDeterministic(t *testing.T) {
	b1 := sampleDraft()
	b1.Status = draw.PoolStatus_POOL_STATUS_DRAFT
	if err := SealPoolDraft(b1, 3); err != nil {
		t.Fatal(err)
	}
	b2 := sampleDraft()
	b2.Status = draw.PoolStatus_POOL_STATUS_DRAFT
	if err := SealPoolDraft(b2, 3); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1.ConfigHash, b2.ConfigHash) {
		t.Fatal("同配置同版本 hash 不一致（确定性序列化失败）")
	}
	b3 := sampleDraft()
	b3.Status = draw.PoolStatus_POOL_STATUS_DRAFT
	b3.Entries[1].Weight = 31
	if err := SealPoolDraft(b3, 3); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(b1.ConfigHash, b3.ConfigHash) {
		t.Fatal("配置变更后 hash 应不同")
	}
}

func TestDraftRequestHash(t *testing.T) {
	b := sampleDraft()
	b.Status = draw.PoolStatus_POOL_STATUS_DRAFT
	h1, err := DraftRequestHash(b)
	if err != nil {
		t.Fatal(err)
	}

	// 幂等键载荷与密封字段无关：携带 version/hash/publishedAt 的克隆应得到相同 hash
	sealed := proto.Clone(b).(*draw.BoxDraft)
	sealed.Version = 7
	sealed.ConfigHash = bytes.Repeat([]byte{1}, 32)
	sealed.PublishedAtUnixMillis = 999
	h2, err := DraftRequestHash(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h1, h2) {
		t.Fatal("密封字段变化不应影响幂等键载荷 hash")
	}

	// 业务字段变化 → hash 变化
	sealed.Entries[0].Quantity = 3
	h3, err := DraftRequestHash(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(h1, h3) {
		t.Fatal("载荷变化后 hash 应不同")
	}
}

func TestMarshalUnmarshalDraftRoundTrip(t *testing.T) {
	b := sampleDraft()
	b.PoolId = 1000001
	b.Status = draw.PoolStatus_POOL_STATUS_DRAFT
	if err := SealPoolDraft(b, 2); err != nil {
		t.Fatal(err)
	}
	b.PublishedAtUnixMillis = 1234567890123

	blob, err := MarshalDraft(b)
	if err != nil {
		t.Fatal(err)
	}
	back, err := UnmarshalDraft(blob)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(back, b) {
		t.Fatalf("序列化往返不一致:\n got=%v\nwant=%v", back, b)
	}
}
