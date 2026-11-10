package core

import (
	"bytes"
	"encoding/json"
	"testing"
)

// server_v2 full_car_test 同款语义案例：确定性、层级对半、座位分配。

func fullCarPrizes() []FullCarSettlementPrize {
	return []FullCarSettlementPrize{
		{Tier: FullCarTierLucky, Weight: 3, PrizeKind: 1, CashFen: 1000},
		{Tier: FullCarTierLucky, Weight: 1, PrizeKind: 2, PointsAmount: 500},
		{Tier: FullCarTierNormal, Weight: 7, PrizeKind: 2, PointsAmount: 100},
		{Tier: FullCarTierNormal, Weight: 1, PrizeKind: 3, CouponTemplateNo: "T-FIX-500"},
	}
}

func TestSettleFullCarDeterministicAndHalfLucky(t *testing.T) {
	// 4 座全坐满：luckyCount=2，4 个奖恰 2 LUCKY（server_v2 TestFullCarDepartureSplitsTiersAndPersistsAwards 同款）
	seats := []int64{10, 20, 30, 40}
	seed := []byte("fixed-seed")
	awards := SettleFullCar(seats, fullCarPrizes(), seed)
	if len(awards) != 4 {
		t.Fatalf("4 seats want 4 awards, got %d", len(awards))
	}
	lucky := 0
	for _, a := range awards {
		if a.Tier == FullCarTierLucky {
			lucky++
		}
	}
	if lucky != 2 {
		t.Fatalf("want exactly 2 lucky awards, got %d: %+v", lucky, awards)
	}
	// 同 state+seed 两次结果字节一致（确定性，可回放）
	again := SettleFullCar(seats, fullCarPrizes(), seed)
	encoded, _ := json.Marshal(again)
	want, _ := json.Marshal(awards)
	if !bytes.Equal(encoded, want) {
		t.Fatalf("settlement is not deterministic:\n%s\n%s", encoded, want)
	}
	// 每个座位的主人保持（座位 → 用户映射不变）
	for i, a := range awards {
		if a.SeatIndex != int32(i) || a.UserID != seats[i] {
			t.Fatalf("award seat/user mismatch at %d: %+v", i, a)
		}
	}
}

func TestSettleFullCarLuckyCountBoundaries(t *testing.T) {
	seats := []int64{1, 2, 3, 4}

	// 全 normal 奖 → luckyCount=0，全部 NORMAL 层
	onlyNormal := []FullCarSettlementPrize{
		{Tier: FullCarTierNormal, Weight: 1, PrizeKind: 2, PointsAmount: 100},
	}
	for _, a := range SettleFullCar(seats, onlyNormal, []byte("s")) {
		if a.Tier != FullCarTierNormal {
			t.Fatalf("only-normal config want all NORMAL, got %+v", a)
		}
	}

	// 全 lucky 奖 → luckyCount=seats，全部 LUCKY 层
	onlyLucky := []FullCarSettlementPrize{
		{Tier: FullCarTierLucky, Weight: 1, PrizeKind: 1, CashFen: 100},
	}
	awards := SettleFullCar(seats, onlyLucky, []byte("s"))
	if len(awards) != 4 {
		t.Fatalf("only-lucky config want 4 awards, got %d", len(awards))
	}
	for _, a := range awards {
		if a.Tier != FullCarTierLucky {
			t.Fatalf("only-lucky config want all LUCKY, got %+v", a)
		}
	}

	// 奇数座 floor：3 座 floor(3/2)=1 幸运
	three := SettleFullCar([]int64{1, 2, 3}, fullCarPrizes(), []byte("s"))
	lucky := 0
	for _, a := range three {
		if a.Tier == FullCarTierLucky {
			lucky++
		}
	}
	if lucky != 1 {
		t.Fatalf("3 seats want 1 lucky, got %d", lucky)
	}

	// weight=0 的奖被过滤（配置垃圾防御）
	filtered := []FullCarSettlementPrize{
		{Tier: FullCarTierNormal, Weight: 0, PrizeKind: 2, PointsAmount: 100}, // 应被过滤
		{Tier: FullCarTierNormal, Weight: 1, PrizeKind: 3, CouponTemplateNo: "T-FIX-500"},
	}
	for _, a := range SettleFullCar(seats, filtered, []byte("s")) {
		if a.Prize.CouponTemplateNo != "T-FIX-500" {
			t.Fatalf("weight=0 prize must be filtered, got %+v", a.Prize)
		}
	}
}

func TestFullCarSeatAssignment(t *testing.T) {
	// 显式指定座（server_v2 TestFullCarOccupiesExplicitAndAutomaticSeats 同款）
	seats := []int64{0, 0, 0}
	if got := AssignFullCarSeat(seats, 2); got != 2 {
		t.Fatalf("explicit seat 2 want 2, got %d", got)
	}
	seats[2] = 10
	// 自动分配 → 首个空位
	if got := AssignFullCarSeat(seats, -1); got != 0 {
		t.Fatalf("auto want first empty seat 0, got %d", got)
	}
	seats[0] = 20
	if got := AssignFullCarSeat(seats, -1); got != 1 {
		t.Fatalf("auto want seat 1, got %d", got)
	}
	// 用户重复占座判定
	if !FullCarContainsUser(seats, 10) {
		t.Fatal("user 10 should be seated")
	}
	if FullCarContainsUser(seats, 99) {
		t.Fatal("user 99 should not be seated")
	}
	// 满座自动分配 → -1（调用方映射 3021/3022）
	seats[1] = 30
	if got := AssignFullCarSeat(seats, -1); got != -1 {
		t.Fatalf("full car auto want -1, got %d", got)
	}
}

func TestFullCarRandomStable(t *testing.T) {
	// 同一 (seed,domain,index) 结果恒定；不同 domain/index 结果不同（漂白）
	a := FullCarRandom([]byte("gid-1"), "tier", 3)
	b := FullCarRandom([]byte("gid-1"), "tier", 3)
	if a != b {
		t.Fatal("FullCarRandom must be stable")
	}
	if FullCarRandom([]byte("gid-1"), "prize", 3) == a {
		t.Fatal("different domain should diverge")
	}
	if FullCarRandom([]byte("gid-2"), "tier", 3) == a {
		t.Fatal("different seed should diverge")
	}
}
