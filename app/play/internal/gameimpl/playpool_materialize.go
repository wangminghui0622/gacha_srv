package gameimpl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gacha_srv/app/play/internal/core"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ══════════════ 统一玩赏池：发布投影 ═══════════════
//
// 统一表 play_pool_* 是配置的单一事实源；发布时按 mode 投影到各玩法
// 现有运行表（config + entries/cells），玩法运行态无需改动。
//
// config_json（camelCase）约定：
//   gashapon     {}
//   fullcar      {"seatCount":4}
//   combochain   {"ticketsPerCar":2,"totalCars":10,"tierTriggerProbabilityBps":300}
//   dig          {"cellCount":9}
//   groupbuy     {"targetBetCount":3,"perBetAwardLimit":0}
//   memorymatch  {"roundsPerPurchase":3,"maxPairCount":3,...,"facePresets":[...],"backPresets":[...]}

func materializePlayPool(db *gorm.DB, v *gamemodelnc.PlayPoolVersion, prizes []gamemodelnc.PlayPoolPrize) error {
	for _, p := range prizes {
		if p.PrizeType == 3 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=400 msg=奖品 %s 使用箱子类型，玩法投影暂不支持", p.Name))
		}
		if p.PrizeType == 2 && v.Mode != "dig" && v.Mode != "memorymatch" && v.Mode != "groupbuy" && v.Mode != "combochain" {
			return status.Error(codes.Aborted, fmt.Sprintf("code=400 msg=奖品 %s 为实物 SKU，当前玩法暂不支持", p.Name))
		}
	}
	// 新版本发布重置运行态（entries 库存已替换；旧 state 会与之不一致）。
	// 历史 records 保留；管理端应在无进行中购买时发布新版本。
	if err := resetPlayRuntimeState(db, v.Mode, v.PoolId); err != nil {
		return err
	}
	switch v.Mode {
	case "gashapon":
		return materializeGashapon(db, v, prizes)
	case "fullcar":
		return materializeFullCar(db, v, prizes)
	case "combochain":
		return materializeComboChain(db, v, prizes)
	case "dig":
		return materializeDig(db, v, prizes)
	case "groupbuy":
		return materializeGroupBuy(db, v, prizes)
	case "memorymatch":
		return materializeMemoryMatch(db, v, prizes)
	default:
		return status.Error(codes.Aborted, "code=400 msg=未知玩法")
	}
}

func resetPlayRuntimeState(db *gorm.DB, mode string, poolId int64) error {
	var err error
	switch mode {
	case "gashapon":
		err = db.Where("config_id = ?", poolId).Delete(&gamemodelnc.GashaponState{}).Error
		if err == nil {
			err = db.Where("config_id = ?", poolId).Delete(&gamemodelnc.GashaponLuckyAssignments{}).Error
		}
	case "fullcar":
		err = db.Where("config_id = ?", poolId).Delete(&gamemodelnc.FullCarState{}).Error
	case "groupbuy":
		err = db.Where("config_id = ?", poolId).Delete(&gamemodelnc.GroupBuyState{}).Error
	case "combochain":
		err = db.Where("config_id = ?", poolId).Delete(&gamemodelnc.ComboChainState{}).Error
	}
	return err
}

func upsertConfig(db *gorm.DB, row interface{}, columns []string) error {
	return db.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns(columns)}).Create(row).Error
}

// ── 扭蛋 ──

func materializeGashapon(db *gorm.DB, v *gamemodelnc.PlayPoolVersion, prizes []gamemodelnc.PlayPoolPrize) error {
	type specialPrize struct {
		PrizeIndex       int32  `json:"prize_index"`
		SpecialKind      string `json:"special_kind"`
		PrizeKind        int64  `json:"prize_kind"`
		CashFen          int64  `json:"cash_fen"`
		PointsAmount     int64  `json:"points_amount"`
		CouponTemplateNo string `json:"coupon_template_no"`
		Quantity         int32  `json:"quantity"`
		StageOrder       int32  `json:"stage_order"`
	}
	specials := make([]specialPrize, 0)
	entries := make([]gamemodel.GashaponEntries, 0)
	for _, p := range prizes {
		if p.SpecialKind != "" {
			kind := playPoolAssetKind(&p)
			if kind == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=400 msg=特别赏 %s 缺少奖金配置", p.Name))
			}
			stage := int32(0)
			if p.SpecialKind == "LUCKY" && p.TargetValue > 0 {
				stage = p.TargetValue
			}
			specials = append(specials, specialPrize{
				PrizeIndex:       int32(len(specials)),
				SpecialKind:      p.SpecialKind,
				PrizeKind:        kind,
				CashFen:          p.CashFen,
				PointsAmount:     p.PointsAmount,
				CouponTemplateNo: p.CouponTemplateNo,
				Quantity:         p.Quantity,
				StageOrder:       stage,
			})
			continue
		}
		entries = append(entries, gamemodel.GashaponEntries{
			ConfigId:  v.PoolId,
			Name:      p.Name,
			Weight:    p.Weight,
			Quantity:  int64(p.Quantity),
			Remaining: int64(p.Remaining),
			IsPrize:   int64(p.IsPrize),
		})
	}
	specialJSON, err := json.Marshal(specials)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := upsertConfig(db, &gamemodel.GashaponConfigs{
		Id:                v.PoolId,
		Name:              v.Name,
		PriceFen:          v.PriceFen,
		SpecialPrizesJson: string(specialJSON),
		Status:            1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, []string{"name", "price_fen", "special_prizes_json", "status", "updated_at"}); err != nil {
		return err
	}
	if err := db.Where("config_id = ?", v.PoolId).Delete(&gamemodel.GashaponEntries{}).Error; err != nil {
		return err
	}
	if len(entries) == 0 {
		return status.Error(codes.Aborted, "code=400 msg=扭蛋没有普通赏")
	}
	if err := db.Create(&entries).Error; err != nil {
		return err
	}

	// 新池首次发布时补运行态；已存在的状态不重置（避免打断进行中的购买）
	var st gamemodelnc.GashaponState
	err = db.Where("config_id = ?", v.PoolId).First(&st).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var total int64
		for _, e := range entries {
			if e.Quantity < 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=400 msg=扭蛋条目 %s 必须限量", e.Name))
			}
			total += e.Quantity
		}
		if total <= 0 {
			return status.Error(codes.Aborted, "code=400 msg=扭蛋总票数必须大于 0")
		}
		return db.Create(&gamemodelnc.GashaponState{
			ConfigId:         v.PoolId,
			ParticipantsJson: "{}",
			RecordsJson:      "[]",
			SentJson:         gashaponSentJSON(make([]int32, len(specials))),
			AwardsJson:       "[]",
			RemainingTickets: total,
			Status:           1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}).Error
	}
	return err
}

// ── 整车 ──

func materializeFullCar(db *gorm.DB, v *gamemodelnc.PlayPoolVersion, prizes []gamemodelnc.PlayPoolPrize) error {
	cfg, err := parseFullCarPoolConfig(v.ConfigJson)
	if err != nil {
		return status.Error(codes.Aborted, "code=400 msg=整车 configJson 非法: "+err.Error())
	}
	settlements := make([]core.FullCarSettlementPrize, 0)
	entries := make([]gamemodel.FullCarEntries, 0)
	for _, p := range prizes {
		if p.SpecialKind == "SETTLEMENT" {
			settlements = append(settlements, core.FullCarSettlementPrize{
				Tier:             p.TargetValue,
				Weight:           uint64(p.Weight),
				PrizeKind:        playPoolAssetKind(&p),
				CashFen:          p.CashFen,
				PointsAmount:     p.PointsAmount,
				CouponTemplateNo: p.CouponTemplateNo,
			})
			continue
		}
		entries = append(entries, gamemodel.FullCarEntries{
			ConfigId:  v.PoolId,
			Name:      p.Name,
			Weight:    p.Weight,
			Quantity:  int64(p.Quantity),
			Remaining: int64(p.Remaining),
			IsPrize:   int64(p.IsPrize),
		})
	}
	settlementJSON, err := json.Marshal(settlements)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := upsertConfig(db, &gamemodel.FullCarConfigs{
		Id:                   v.PoolId,
		Name:                 v.Name,
		PriceFen:             v.PriceFen,
		SeatCount:            cfg.SeatCount,
		SettlementPrizesJson: string(settlementJSON),
		Status:               1,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, []string{"name", "price_fen", "seat_count", "settlement_prizes_json", "status", "updated_at"}); err != nil {
		return err
	}
	if err := db.Where("config_id = ?", v.PoolId).Delete(&gamemodel.FullCarEntries{}).Error; err != nil {
		return err
	}
	if len(entries) == 0 {
		return status.Error(codes.Aborted, "code=400 msg=整车没有普通条目")
	}
	return db.Create(&entries).Error
}

// ── 连击链（配置 JSON 型，无 entries 表）──

func materializeComboChain(db *gorm.DB, v *gamemodelnc.PlayPoolVersion, prizes []gamemodelnc.PlayPoolPrize) error {
	cfg, err := parseComboChainPoolConfig(v.ConfigJson)
	if err != nil {
		return status.Error(codes.Aborted, "code=400 msg=连击链 configJson 非法: "+err.Error())
	}
	globals := make([]core.ComboChainGlobalSlot, 0)
	shared := make([]core.ComboChainSharedPrize, 0)
	for _, p := range prizes {
		kind := playPoolPrizeKind(&p)
		itemName := ""
		if kind == prizeGoods {
			itemName = p.Name
		}
		shape := core.PrizeShape{
			PrizeKind:        kind,
			CashFen:          p.CashFen,
			PointsAmount:     p.PointsAmount,
			CouponTemplateNo: p.CouponTemplateNo,
			SkuId:            p.SkuId,
			ItemName:         itemName,
		}
		switch p.SpecialKind {
		case "GLOBAL":
			globals = append(globals, core.ComboChainGlobalSlot{Index: int(p.TargetValue), PrizeShape: shape})
		case "COMBO_TIER", "EMPTY_STREAK", "WIN_STREAK":
			extra := struct {
				TierWeight           int32 `json:"tierWeight"`
				ConcentrateThreshold int32 `json:"concentrateThreshold"`
				StreakThreshold      int32 `json:"streakThreshold"`
			}{}
			if strings.TrimSpace(p.ExtraJson) != "" {
				if err := json.Unmarshal([]byte(p.ExtraJson), &extra); err != nil {
					return status.Error(codes.Aborted, fmt.Sprintf("code=400 msg=共享奖 %s 的 extraJson 非法: %v", p.Name, err))
				}
			}
			shared = append(shared, core.ComboChainSharedPrize{
				Kind:                 p.SpecialKind,
				Quantity:             p.Quantity,
				TierWeight:           extra.TierWeight,
				ConcentrateThreshold: extra.ConcentrateThreshold,
				StreakThreshold:      extra.StreakThreshold,
				PrizeShape:           shape,
			})
		default:
			return status.Error(codes.Aborted, fmt.Sprintf("code=400 msg=连击链奖品 %s 类别非法", p.Name))
		}
	}
	globalJSON, err := json.Marshal(globals)
	if err != nil {
		return err
	}
	sharedJSON, err := json.Marshal(shared)
	if err != nil {
		return err
	}
	now := time.Now()
	return upsertConfig(db, &gamemodel.ComboChainConfigs{
		Id:                        v.PoolId,
		Name:                      v.Name,
		PriceFen:                  v.PriceFen,
		TicketsPerCar:             cfg.TicketsPerCar,
		TotalCars:                 cfg.TotalCars,
		GlobalSlotsJson:           string(globalJSON),
		SharedPrizesJson:          string(sharedJSON),
		TierTriggerProbabilityBps: cfg.TierTriggerProbabilityBps,
		Status:                    1,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}, []string{"name", "price_fen", "tickets_per_car", "total_cars", "global_slots_json", "shared_prizes_json", "tier_trigger_probability_bps", "status", "updated_at"})
}

// ── 挖宝箱 ──

func materializeDig(db *gorm.DB, v *gamemodelnc.PlayPoolVersion, prizes []gamemodelnc.PlayPoolPrize) error {
	cfg, err := parseDigPoolConfig(v.ConfigJson)
	if err != nil {
		return status.Error(codes.Aborted, "code=400 msg=挖宝 configJson 非法: "+err.Error())
	}
	cells := make([]gamemodel.DigBoxCells, 0, len(prizes))
	for _, p := range prizes {
		kind := playPoolAssetKind(&p)
		if p.PrizeType == 2 {
			kind = prizeGoods
		}
		cells = append(cells, gamemodel.DigBoxCells{
			ConfigId:         v.PoolId,
			CellIndex:        int64(p.TargetValue),
			PrizeKind:        kind,
			CashFen:          p.CashFen,
			PointsAmount:     p.PointsAmount,
			CouponTemplateNo: p.CouponTemplateNo,
			SkuId:            p.SkuId,
			ItemName:         p.Name,
		})
	}
	now := time.Now()
	if err := upsertConfig(db, &gamemodel.DigBoxConfigs{
		Id:        v.PoolId,
		Name:      v.Name,
		PriceFen:  v.PriceFen,
		CellCount: cfg.CellCount,
		Status:    1,
		CreatedAt: now,
		UpdatedAt: now,
	}, []string{"name", "price_fen", "cell_count", "status", "updated_at"}); err != nil {
		return err
	}
	if err := db.Where("config_id = ?", v.PoolId).Delete(&gamemodel.DigBoxCells{}).Error; err != nil {
		return err
	}
	if len(cells) == 0 {
		return status.Error(codes.Aborted, "code=400 msg=挖宝没有格子")
	}
	return db.Create(&cells).Error
}

// ── 拼团 ──

// groupBuyNormalPrizeKind 普通条目自带资产：仅实物（SKU）在普通槽发奖；
// 虚拟资产继续走子奖（parent_entry_id）语义，保持旧配置行为。
func groupBuyNormalPrizeKind(p *gamemodelnc.PlayPoolPrize) int64 {
	if p.PrizeType == 2 {
		return prizeGoods
	}
	return 0
}

func materializeGroupBuy(db *gorm.DB, v *gamemodelnc.PlayPoolVersion, prizes []gamemodelnc.PlayPoolPrize) error {
	cfg, err := parseGroupBuyPoolConfig(v.ConfigJson)
	if err != nil {
		return status.Error(codes.Aborted, "code=400 msg=拼团 configJson 非法: "+err.Error())
	}
	type luckyJSON struct {
		PrizeIndex       int64  `json:"prize_index"`
		PrizeKind        int64  `json:"prize_kind"`
		CashFen          int64  `json:"cash_fen"`
		PointsAmount     int64  `json:"points_amount"`
		CouponTemplateNo string `json:"coupon_template_no"`
		SkuId            int64  `json:"sku_id,omitempty"`
		ItemName         string `json:"item_name,omitempty"`
		Quantity         int32  `json:"quantity"`
	}
	type luckyChildJSON struct {
		LuckyIndex       int64  `json:"lucky_index"`
		PrizeKind        int64  `json:"prize_kind"`
		CashFen          int64  `json:"cash_fen"`
		PointsAmount     int64  `json:"points_amount"`
		CouponTemplateNo string `json:"coupon_template_no"`
		SkuId            int64  `json:"sku_id,omitempty"`
		ItemName         string `json:"item_name,omitempty"`
		Quantity         int32  `json:"quantity"`
	}

	// 父奖 prizeIndex -> lucky_prizes_json 下标
	luckyOrdinal := map[int32]int32{}
	lucky := make([]luckyJSON, 0)
	children := make([]luckyChildJSON, 0)
	for _, p := range prizes {
		if p.SpecialKind == "LUCKY" {
			kind := playPoolAssetKind(&p)
			if p.PrizeType == 2 {
				kind = prizeGoods
			}
			luckyOrdinal[p.PrizeIndex] = int32(len(lucky))
			lucky = append(lucky, luckyJSON{
				PrizeIndex:       int64(len(lucky)),
				PrizeKind:        kind,
				CashFen:          p.CashFen,
				PointsAmount:     p.PointsAmount,
				CouponTemplateNo: p.CouponTemplateNo,
				SkuId:            p.SkuId,
				ItemName:         p.Name,
				Quantity:         p.Quantity,
			})
		}
	}
	for _, p := range prizes {
		if p.SpecialKind != "LUCKY_CHILD" {
			continue
		}
		ordinal, ok := luckyOrdinal[int32(p.TargetValue)]
		if !ok {
			return status.Error(codes.Aborted, fmt.Sprintf("code=400 msg=幸运子奖 %s 指向的幸运奖不存在", p.Name))
		}
		children = append(children, luckyChildJSON{
			LuckyIndex:       int64(ordinal),
			PrizeKind:        playPoolPrizeKind(&p),
			CashFen:          p.CashFen,
			PointsAmount:     p.PointsAmount,
			CouponTemplateNo: p.CouponTemplateNo,
			SkuId:            p.SkuId,
			ItemName:         p.Name,
			Quantity:         p.Quantity,
		})
	}
	luckyJSONBytes, err := json.Marshal(lucky)
	if err != nil {
		return err
	}
	childrenJSONBytes, err := json.Marshal(children)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := upsertConfig(db, &gamemodel.GroupBuyConfigs{
		Id:                v.PoolId,
		Name:              v.Name,
		PriceFen:          v.PriceFen,
		TargetBetCount:    cfg.TargetBetCount,
		PerBetAwardLimit:  cfg.PerBetAwardLimit,
		LuckyPrizesJson:   string(luckyJSONBytes),
		LuckyChildrenJson: string(childrenJSONBytes),
		Status:            1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, []string{"name", "price_fen", "target_bet_count", "per_bet_award_limit", "lucky_prizes_json", "lucky_children_json", "status", "updated_at"}); err != nil {
		return err
	}
	if err := db.Where("config_id = ?", v.PoolId).Delete(&gamemodel.GroupBuyEntries{}).Error; err != nil {
		return err
	}

	// 先写父条目（拿到自增 ID），再写子奖（parent_entry_id）
	parentIDs := map[int32]int64{}
	for i := range prizes {
		p := &prizes[i]
		if p.SpecialKind != "" || p.ParentIndex >= 0 {
			continue
		}
		row := &gamemodel.GroupBuyEntries{
			ConfigId:  v.PoolId,
			Name:      p.Name,
			Weight:    p.Weight,
			Quantity:  int64(p.Quantity),
			Remaining: int64(p.Remaining),
			IsPrize:   int64(p.IsPrize),
			PrizeKind: groupBuyNormalPrizeKind(p),
			SkuId:     p.SkuId,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.Create(row).Error; err != nil {
			return err
		}
		parentIDs[p.PrizeIndex] = row.Id
	}
	for i := range prizes {
		p := &prizes[i]
		if p.SpecialKind != "" || p.ParentIndex < 0 {
			continue
		}
		parentID, ok := parentIDs[p.ParentIndex]
		if !ok {
			return status.Error(codes.Aborted, fmt.Sprintf("code=400 msg=拼团子奖 %s 指向的父条目不存在", p.Name))
		}
		if err := db.Create(&gamemodel.GroupBuyEntries{
			ConfigId:         v.PoolId,
			Name:             p.Name,
			Weight:           0,
			Quantity:         int64(p.Quantity),
			Remaining:        int64(p.Remaining),
			IsPrize:          int64(p.IsPrize),
			ParentEntryId:    parentID,
			PrizeKind:        playPoolPrizeKind(p),
			CashFen:          p.CashFen,
			PointsAmount:     p.PointsAmount,
			CouponTemplateNo: p.CouponTemplateNo,
			SkuId:            p.SkuId,
			CreatedAt:        now,
			UpdatedAt:        now,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ── 记忆翻牌 ──

func materializeMemoryMatch(db *gorm.DB, v *gamemodelnc.PlayPoolVersion, prizes []gamemodelnc.PlayPoolPrize) error {
	cfg, err := parseMemoryMatchPoolConfig(v.ConfigJson)
	if err != nil {
		return status.Error(codes.Aborted, "code=400 msg=记忆翻牌 configJson 非法: "+err.Error())
	}
	weightsJSON, err := json.Marshal(cfg.PatternWeights)
	if err != nil {
		return err
	}
	facePresets := string(cfg.FacePresets)
	if facePresets == "" || facePresets == "null" {
		facePresets = "[]"
	}
	backPresets := string(cfg.BackPresets)
	if backPresets == "" || backPresets == "null" {
		backPresets = "[]"
	}
	entries := make([]gamemodel.MemoryMatchEntries, 0, len(prizes))
	for _, p := range prizes {
		kind := playPoolAssetKind(&p)
		if p.PrizeType == 2 {
			kind = prizeGoods
		}
		entries = append(entries, gamemodel.MemoryMatchEntries{
			ConfigId:         v.PoolId,
			Name:             p.Name,
			TotalPairCount:   int64(p.TargetValue),
			Weight:           p.Weight,
			PrizeKind:        kind,
			CashFen:          p.CashFen,
			PointsAmount:     p.PointsAmount,
			CouponTemplateNo: p.CouponTemplateNo,
			SkuId:            p.SkuId,
			Quantity:         int64(p.Quantity),
			Remaining:        int64(p.Remaining),
			IsPrize:          int64(p.IsPrize),
		})
	}
	now := time.Now()
	if err := upsertConfig(db, &gamemodel.MemoryMatchConfigs{
		Id:                    v.PoolId,
		Name:                  v.Name,
		RoundsPerPurchase:     cfg.RoundsPerPurchase,
		MaxPairCount:          cfg.MaxPairCount,
		PurchasePriceMinor:    v.PriceFen,
		WalletKind:            cfg.WalletKind,
		SecondDestinyEnabled:  cfg.SecondDestinyEnabled,
		DeckStrategy:          cfg.DeckStrategy,
		PatternWeightsJson:    string(weightsJSON),
		ExactMinPairCount:     cfg.ExactMinPairCount,
		ExactMaxPairCount:     cfg.ExactMaxPairCount,
		ExactAttemptLimit:     cfg.ExactAttemptLimit,
		FacePresetsJson:       facePresets,
		BackPresetsJson:       backPresets,
		DefaultFacePresetId:   cfg.DefaultFacePresetId,
		DefaultBackPresetId:   cfg.DefaultBackPresetId,
		ChampionMinTotalPairs: cfg.ChampionMinTotalPairs,
		ChampionShareBps:      cfg.ChampionShareBps,
		Status:                1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}, []string{"name", "rounds_per_purchase", "max_pair_count", "purchase_price_minor", "wallet_kind", "second_destiny_enabled", "deck_strategy", "pattern_weights_json", "exact_min_pair_count", "exact_max_pair_count", "exact_attempt_limit", "face_presets_json", "back_presets_json", "default_face_preset_id", "default_back_preset_id", "champion_min_total_pairs", "champion_share_bps", "status", "updated_at"}); err != nil {
		return err
	}
	if err := db.Where("config_id = ?", v.PoolId).Delete(&gamemodel.MemoryMatchEntries{}).Error; err != nil {
		return err
	}
	if len(entries) == 0 {
		return status.Error(codes.Aborted, "code=400 msg=记忆翻牌没有奖档")
	}
	return db.Create(&entries).Error
}
