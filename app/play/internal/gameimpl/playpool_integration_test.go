package gameimpl

import (
	"context"
	"os"
	"testing"
	"time"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestPlayPoolIntegration 需要真实 MySQL：
//
//	PLAYPOOL_TEST_DSN='root:we7root@tcp(127.0.0.1:3306)/gacha_play?charset=utf8mb4&parseTime=True&loc=Local' \
//	go test ./app/play/internal/gameimpl -run 'TestPlayPool' -v
func TestPlayPoolIntegration(t *testing.T) {
	svcCtx, db := playPoolTestContext(t)
	ctx := context.Background()
	key := "test-playpool-" + time.Now().Format("20060102150405")
	prizes := []*play.PlayPoolPrize{
		{PrizeIndex: 0, PrizeType: 1, Name: "普通赏", Weight: 10, Quantity: 10, IsPrize: true},
		{PrizeIndex: 1, PrizeType: 1, Name: "LAST赏", SpecialKind: "LAST", CashFen: 500, Quantity: 1, IsPrize: true},
	}
	in := &play.CreatePlayPoolDraftReq{
		IdempotencyKey: key,
		Mode:           "gashapon",
		Name:           "集成测试池",
		PriceFen:       100,
		ConfigJson:     `{"source":"integration"}`,
		Prizes:         prizes,
	}

	draft, err := CreatePlayPoolDraft(ctx, svcCtx, in)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.PoolId < 1000000 || draft.Version != 1 || draft.Status != "DRAFT" {
		t.Fatalf("unexpected draft: %+v", draft)
	}
	poolId := draft.PoolId
	t.Cleanup(func() { cleanupPlayPoolTest(t, db, poolId) })

	// 幂等：同 key 同请求返回原草稿
	again, err := CreatePlayPoolDraft(ctx, svcCtx, in)
	if err != nil || again.PoolId != poolId || again.Version != 1 {
		t.Fatalf("idempotent draft = %+v, err=%v", again, err)
	}

	if _, err := PublishPlayPool(ctx, svcCtx, &play.PublishPlayPoolReq{PoolId: poolId, Version: 1}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	got, err := GetPlayPoolVersion(ctx, svcCtx, &play.GetPlayPoolVersionReq{PoolId: poolId, Version: 1})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != 2 || len(got.Prizes) != 2 || got.Prizes[1].SpecialKind != "LAST" {
		t.Fatalf("unexpected version: %+v", got)
	}

	// gashapon 投影
	var cfg struct {
		Id                int64
		SpecialPrizesJson string
	}
	if err := db.Table("gashapon_configs").Select("id, special_prizes_json").Where("id = ?", poolId).Scan(&cfg).Error; err != nil {
		t.Fatalf("query gashapon config: %v", err)
	}
	if cfg.Id != poolId || cfg.SpecialPrizesJson == "" {
		t.Fatalf("gashapon config not materialized: %+v", cfg)
	}
	if n := tableCount(t, db, "gashapon_entries", poolId); n != 1 {
		t.Fatalf("gashapon entries = %d, want 1", n)
	}

	list, err := ListPlayPools(ctx, svcCtx, &play.ListPlayPoolsReq{AfterPoolId: poolId - 1, PageSize: 10, Mode: "gashapon"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, it := range list.Items {
		if it.PoolId == poolId && it.Status == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("published pool not in list: %+v", list.Items)
	}
}

// TestPlayPoolMaterializeAllModes 校验 6 个玩法都能从统一池投影到运行表。
func TestPlayPoolMaterializeAllModes(t *testing.T) {
	svcCtx, db := playPoolTestContext(t)
	ctx := context.Background()

	cases := []struct {
		mode       string
		configJSON string
		prizes     []*play.PlayPoolPrize
		table      string // 运行表
		wantRows   int64
	}{
		{
			mode:       "fullcar",
			configJSON: `{"seatCount":4}`,
			prizes: []*play.PlayPoolPrize{
				{PrizeIndex: 0, Name: "谢谢参与", Weight: 30, Quantity: -1, IsPrize: false},
				{PrizeIndex: 1, Name: "金券", Weight: 20, Quantity: 100, IsPrize: true},
				{PrizeIndex: 2, Name: "幸运结算", SpecialKind: "SETTLEMENT", TargetValue: 1, Weight: 1, CashFen: 1000, Quantity: 0},
				{PrizeIndex: 3, Name: "普通结算", SpecialKind: "SETTLEMENT", TargetValue: 2, Weight: 2, PointsAmount: 100, Quantity: 0},
			},
			table: "full_car_entries", wantRows: 2,
		},
		{
			mode:       "dig",
			configJSON: `{"cellCount":4}`,
			prizes: []*play.PlayPoolPrize{
				{PrizeIndex: 0, Name: "现金格", TargetValue: 0, CashFen: 50, IsPrize: true},
				{PrizeIndex: 1, Name: "空格", TargetValue: 1, IsPrize: false},
				{PrizeIndex: 2, Name: "实物手办", PrizeType: 2, SkuId: 9001, TargetValue: 2, IsPrize: true},
			},
			table: "dig_box_cells", wantRows: 3,
		},
		{
			mode:       "groupbuy",
			configJSON: `{"targetBetCount":2}`,
			prizes: []*play.PlayPoolPrize{
				{PrizeIndex: 0, Name: "谢谢参与", Weight: 30, Quantity: 2, IsPrize: false},
				{PrizeIndex: 1, Name: "普通奖", Weight: 20, Quantity: 2, IsPrize: true},
				{PrizeIndex: 2, Name: "普通子奖", ParentIndex: 1, HasParent: true, PointsAmount: 50, Quantity: 1, IsPrize: true},
				{PrizeIndex: 3, Name: "幸运奖", SpecialKind: "LUCKY", CashFen: 500, Quantity: 1, IsPrize: true},
				{PrizeIndex: 4, Name: "幸运子奖", SpecialKind: "LUCKY_CHILD", TargetValue: 3, PointsAmount: 20, Quantity: 1, IsPrize: true},
			},
			table: "group_buy_entries", wantRows: 3,
		},
		{
			mode: "memorymatch",
			configJSON: `{"roundsPerPurchase":3,"maxPairCount":3,"patternWeights":[1,1,1,1,1,1,1,1,1],` +
				`"facePresets":[{"presetId":1,"name":"默认","faces":[]}],"backPresets":[{"presetId":1,"name":"默认","backImageUrl":"","faceBackgroundImageUrl":""}]}`,
			prizes: []*play.PlayPoolPrize{
				{PrizeIndex: 0, Name: "9对大奖", TargetValue: 3, Weight: 1, CashFen: 500, Quantity: 10, IsPrize: true},
				{PrizeIndex: 1, Name: "谢谢参与", TargetValue: 0, Weight: 1, Quantity: -1, IsPrize: false},
			},
			table: "memory_match_entries", wantRows: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			key := "test-" + tc.mode + "-" + time.Now().Format("20060102150405")
			draft, err := CreatePlayPoolDraft(ctx, svcCtx, &play.CreatePlayPoolDraftReq{
				IdempotencyKey: key,
				Mode:           tc.mode,
				Name:           tc.mode + "测试池",
				PriceFen:       100,
				ConfigJson:     tc.configJSON,
				Prizes:         tc.prizes,
			})
			if err != nil {
				t.Fatalf("create draft: %v", err)
			}
			t.Cleanup(func() { cleanupPlayPoolTest(t, db, draft.PoolId) })
			if _, err := PublishPlayPool(ctx, svcCtx, &play.PublishPlayPoolReq{PoolId: draft.PoolId, Version: 1}); err != nil {
				t.Fatalf("publish: %v", err)
			}
			if n := tableCount(t, db, tc.table, draft.PoolId); n != tc.wantRows {
				t.Fatalf("%s rows = %d, want %d", tc.table, n, tc.wantRows)
			}
			if tc.mode == "dig" {
				var cell struct {
					PrizeKind int64
					SkuId     int64
					ItemName  string
				}
				if err := db.Table("dig_box_cells").
					Select("prize_kind, sku_id, item_name").
					Where("config_id = ? AND cell_index = ?", draft.PoolId, 2).Scan(&cell).Error; err != nil {
					t.Fatalf("query sku cell: %v", err)
				}
				if cell.PrizeKind != 5 || cell.SkuId != 9001 || cell.ItemName != "实物手办" {
					t.Fatalf("sku cell not projected: %+v", cell)
				}
			}
		})
	}

	// 连击链是纯配置 JSON，无 entries 表
	t.Run("combochain", func(t *testing.T) {
		draft, err := CreatePlayPoolDraft(ctx, svcCtx, &play.CreatePlayPoolDraftReq{
			IdempotencyKey: "test-combochain-" + time.Now().Format("20060102150405"),
			Mode:           "combochain",
			Name:           "连击链测试池",
			PriceFen:       100,
			ConfigJson:     `{"ticketsPerCar":2,"totalCars":2,"tierTriggerProbabilityBps":300}`,
			Prizes: []*play.PlayPoolPrize{
				{PrizeIndex: 0, Name: "全局奖", SpecialKind: "GLOBAL", TargetValue: 0, CashFen: 100, IsPrize: true},
				{PrizeIndex: 1, Name: "连击奖", SpecialKind: "COMBO_TIER", Weight: 1, CashFen: 50, Quantity: 2, ExtraJson: `{"tierWeight":1}`},
			},
		})
		if err != nil {
			t.Fatalf("create draft: %v", err)
		}
		t.Cleanup(func() { cleanupPlayPoolTest(t, db, draft.PoolId) })
		if _, err := PublishPlayPool(ctx, svcCtx, &play.PublishPlayPoolReq{PoolId: draft.PoolId, Version: 1}); err != nil {
			t.Fatalf("publish: %v", err)
		}
		var cfg struct {
			GlobalSlotsJson  string
			SharedPrizesJson string
		}
		if err := db.Table("combo_chain_configs").Select("global_slots_json, shared_prizes_json").
			Where("id = ?", draft.PoolId).Scan(&cfg).Error; err != nil {
			t.Fatalf("query combo config: %v", err)
		}
		if cfg.GlobalSlotsJson == "" || cfg.SharedPrizesJson == "" {
			t.Fatalf("combo config not materialized: %+v", cfg)
		}
	})
}

func playPoolTestContext(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("PLAYPOOL_TEST_DSN")
	if dsn == "" {
		t.Skip("PLAYPOOL_TEST_DSN 未设置")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	return &svc.ServiceContext{DB: db}, db
}

func tableCount(t *testing.T, db *gorm.DB, table string, configId int64) int64 {
	t.Helper()
	var n int64
	if err := db.Table(table).Where("config_id = ?", configId).Count(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func cleanupPlayPoolTest(t *testing.T, db *gorm.DB, poolId int64) {
	t.Helper()
	entryTables := []string{
		"gashapon_entries", "full_car_entries", "dig_box_cells", "group_buy_entries", "memory_match_entries",
	}
	configTables := []string{
		"gashapon_configs", "full_car_configs", "dig_box_configs", "group_buy_configs", "memory_match_configs", "combo_chain_configs",
	}
	for _, table := range entryTables {
		db.Exec("DELETE FROM "+table+" WHERE config_id = ?", poolId)
	}
	for _, table := range configTables {
		db.Exec("DELETE FROM "+table+" WHERE id = ?", poolId)
	}
	db.Exec("DELETE FROM play_pool_prizes WHERE pool_id = ?", poolId)
	db.Exec("DELETE FROM play_pool_versions WHERE pool_id = ?", poolId)
	db.Exec("DELETE FROM play_pool_drafts WHERE pool_id = ?", poolId)
	db.Exec("DELETE FROM play_pool_heads WHERE id = ?", poolId)
	db.Exec("DELETE FROM gashapon_state WHERE config_id = ?", poolId)
}
