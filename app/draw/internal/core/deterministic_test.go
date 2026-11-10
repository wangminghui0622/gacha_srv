package core

import (
	"bytes"
	"testing"
)

// rustFixture 对齐 server_v2 lib.rs 的测试 fixture：
//
//	request_id "draw-1"、pool_id 9、pool_version 3、seed "11"×32、
//	count 4、quantity_weighted=false、entries [{1,10,2},{2,1,2}]。
func rustFixtureEntries() []DeterministicEntry {
	return []DeterministicEntry{
		{EntryID: 1, Weight: 10, Remaining: 2},
		{EntryID: 2, Weight: 1, Remaining: 2},
	}
}

func rustSeed() []byte {
	return bytes.Repeat([]byte{0x11}, 32)
}

func TestDeterministicReplay(t *testing.T) {
	a, ha, err := DrawDeterministic(rustFixtureEntries(), rustSeed(), 9, 3, "draw-1", 4, false)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	b, hb, err := DrawDeterministic(rustFixtureEntries(), rustSeed(), 9, 3, "draw-1", 4, false)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	if ha != hb || len(a) != len(b) {
		t.Fatalf("non-deterministic: %q vs %q", ha, hb)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("outcome %d mismatch: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestDeterministicNeverExceedsRemaining(t *testing.T) {
	out, _, err := DrawDeterministic(rustFixtureEntries(), rustSeed(), 9, 3, "draw-1", 4, false)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	if len(out) != 4 {
		t.Fatalf("expected 4 outcomes, got %d", len(out))
	}
	var count1 int
	for _, o := range out {
		if o.EntryID == 1 {
			count1++
		}
	}
	if count1 != 2 {
		t.Fatalf("entry 1 should appear exactly 2 times (remaining=2), got %d", count1)
	}
}

func TestDeterministicRejectsExhaustion(t *testing.T) {
	_, _, err := DrawDeterministic(rustFixtureEntries(), rustSeed(), 9, 3, "draw-1", 5, false)
	if err != errDeterministicExhaust {
		t.Fatalf("expected exhaustion, got %v", err)
	}
}

func TestDeterministicRejectsDuplicateEntry(t *testing.T) {
	entries := append(rustFixtureEntries(), DeterministicEntry{EntryID: 1, Weight: 1, Remaining: 1})
	_, _, err := DrawDeterministic(entries, rustSeed(), 9, 3, "draw-1", 1, false)
	if err == nil {
		t.Fatal("expected duplicate entry error")
	}
}

// TestAuditHashMatchesRustEngine 用 server_v2 lib.rs 的 fixture 验证 audit hash 字节级一致。
// 期望值由 Rust 引擎（luckybox-draw-engine）对同一 fixture 计算得出。
func TestAuditHashMatchesRustEngine(t *testing.T) {
	_, audit, err := DrawDeterministic(rustFixtureEntries(), rustSeed(), 9, 3, "draw-1", 4, false)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	const want = "fd24e170f99282164d0b8b995b5ebb997c4f1c4176cad4961da584d56c8d79a4"
	if audit != want {
		t.Fatalf("audit hash mismatch: got %s want %s", audit, want)
	}
}

func TestDeterministicFixedWeightsIgnoreStock(t *testing.T) {
	// 固定权重模式：remaining 不影响抽取结果（除非耗尽）。
	a := rustFixtureEntries()
	b := []DeterministicEntry{{EntryID: 1, Weight: 10, Remaining: 10000}, {EntryID: 2, Weight: 1, Remaining: 2}}
	oa, _, err := DrawDeterministic(a, rustSeed(), 9, 3, "draw-1", 1, false)
	if err != nil {
		t.Fatalf("draw a failed: %v", err)
	}
	ob, _, err := DrawDeterministic(b, rustSeed(), 9, 3, "draw-1", 1, false)
	if err != nil {
		t.Fatalf("draw b failed: %v", err)
	}
	if oa[0].EntryID != ob[0].EntryID {
		t.Fatalf("fixed weights should ignore stock until exhausted: %d vs %d", oa[0].EntryID, ob[0].EntryID)
	}
}
