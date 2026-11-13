package gameimpl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ══════════════ 统一玩赏池：校验与配置解析 ═══════════════

var (
	validPlayModes        = map[string]bool{"gashapon": true, "fullcar": true, "combochain": true, "dig": true, "groupbuy": true, "memorymatch": true}
	playPoolSpecialKindRe = regexp.MustCompile(`^[A-Z0-9_]{1,16}$`)
)

func abortf(format string, args ...interface{}) error {
	return status.Error(codes.Aborted, "code=400 msg="+fmt.Sprintf(format, args...))
}

// ── 各玩法 config_json DTO（camelCase；缺省值在 parse 内补齐）──

type fullCarPoolConfig struct {
	SeatCount int64 `json:"seatCount"`
}

type comboChainPoolConfig struct {
	TicketsPerCar             int64 `json:"ticketsPerCar"`
	TotalCars                 int64 `json:"totalCars"`
	TierTriggerProbabilityBps int64 `json:"tierTriggerProbabilityBps"`
}

type digPoolConfig struct {
	CellCount int64 `json:"cellCount"`
}

type groupBuyPoolConfig struct {
	TargetBetCount   int64 `json:"targetBetCount"`
	PerBetAwardLimit int64 `json:"perBetAwardLimit"`
}

type memoryMatchPoolConfig struct {
	RoundsPerPurchase     int64           `json:"roundsPerPurchase"`
	MaxPairCount          int64           `json:"maxPairCount"`
	WalletKind            int64           `json:"walletKind"`
	SecondDestinyEnabled  int64           `json:"secondDestinyEnabled"`
	DeckStrategy          int64           `json:"deckStrategy"`
	PatternWeights        []int64         `json:"patternWeights"`
	ExactMinPairCount     int64           `json:"exactMinPairCount"`
	ExactMaxPairCount     int64           `json:"exactMaxPairCount"`
	ExactAttemptLimit     int64           `json:"exactAttemptLimit"`
	FacePresets           json.RawMessage `json:"facePresets"`
	BackPresets           json.RawMessage `json:"backPresets"`
	DefaultFacePresetId   int64           `json:"defaultFacePresetId"`
	DefaultBackPresetId   int64           `json:"defaultBackPresetId"`
	ChampionMinTotalPairs int64           `json:"championMinTotalPairs"`
	ChampionShareBps      int64           `json:"championShareBps"`
}

func jsonOrDefault(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	return raw
}

func parseFullCarPoolConfig(raw string) (fullCarPoolConfig, error) {
	var c fullCarPoolConfig
	if err := json.Unmarshal([]byte(jsonOrDefault(raw)), &c); err != nil {
		return c, err
	}
	if c.SeatCount < 2 || c.SeatCount > 1000 {
		return c, fmt.Errorf("seatCount 必须在 2..1000")
	}
	return c, nil
}

func parseComboChainPoolConfig(raw string) (comboChainPoolConfig, error) {
	var c comboChainPoolConfig
	if err := json.Unmarshal([]byte(jsonOrDefault(raw)), &c); err != nil {
		return c, err
	}
	if c.TicketsPerCar < 2 || c.TicketsPerCar > 1000 {
		return c, fmt.Errorf("ticketsPerCar 必须在 2..1000")
	}
	if c.TotalCars < 1 || c.TotalCars > 10000 {
		return c, fmt.Errorf("totalCars 必须在 1..10000")
	}
	if c.TierTriggerProbabilityBps < 0 || c.TierTriggerProbabilityBps > 10000 {
		return c, fmt.Errorf("tierTriggerProbabilityBps 必须在 0..10000")
	}
	return c, nil
}

func parseDigPoolConfig(raw string) (digPoolConfig, error) {
	var c digPoolConfig
	if err := json.Unmarshal([]byte(jsonOrDefault(raw)), &c); err != nil {
		return c, err
	}
	if c.CellCount < 1 || c.CellCount > 62 {
		return c, fmt.Errorf("cellCount 必须在 1..62")
	}
	return c, nil
}

func parseGroupBuyPoolConfig(raw string) (groupBuyPoolConfig, error) {
	var c groupBuyPoolConfig
	if err := json.Unmarshal([]byte(jsonOrDefault(raw)), &c); err != nil {
		return c, err
	}
	if c.TargetBetCount < 2 || c.TargetBetCount > 1000 {
		return c, fmt.Errorf("targetBetCount 必须在 2..1000")
	}
	if c.PerBetAwardLimit < 0 {
		return c, fmt.Errorf("perBetAwardLimit 不能为负")
	}
	return c, nil
}

func parseMemoryMatchPoolConfig(raw string) (memoryMatchPoolConfig, error) {
	var c memoryMatchPoolConfig
	if err := json.Unmarshal([]byte(jsonOrDefault(raw)), &c); err != nil {
		return c, err
	}
	if c.WalletKind == 0 {
		c.WalletKind = 1
	}
	if c.DeckStrategy == 0 {
		c.DeckStrategy = 1
	}
	if c.ExactAttemptLimit == 0 {
		c.ExactAttemptLimit = 32
	}
	if c.DefaultFacePresetId == 0 {
		c.DefaultFacePresetId = 1
	}
	if c.DefaultBackPresetId == 0 {
		c.DefaultBackPresetId = 1
	}
	if c.RoundsPerPurchase < 1 || c.RoundsPerPurchase > 100 {
		return c, fmt.Errorf("roundsPerPurchase 必须在 1..100")
	}
	if c.MaxPairCount < 1 || c.MaxPairCount > 27 {
		return c, fmt.Errorf("maxPairCount 必须在 1..27")
	}
	if c.DeckStrategy == 1 && len(c.PatternWeights) != 9 {
		return c, fmt.Errorf("patternWeights 必须是 9 个权重")
	}
	if len(c.FacePresets) == 0 || string(c.FacePresets) == "null" {
		return c, fmt.Errorf("facePresets 必填")
	}
	if len(c.BackPresets) == 0 || string(c.BackPresets) == "null" {
		return c, fmt.Errorf("backPresets 必填")
	}
	return c, nil
}

// ── 奖品通用/分玩法校验 ──

func playPoolAssetKind(p *gamemodelnc.PlayPoolPrize) int64 {
	switch {
	case p.CashFen > 0:
		return 1
	case p.PointsAmount > 0:
		return 2
	case p.CouponTemplateNo != "":
		return 3
	default:
		return 0
	}
}

func validatePlayPoolDraft(in *play.CreatePlayPoolDraftReq) error {
	if !validPlayModes[in.Mode] {
		return abortf("未知玩法")
	}
	if strings.TrimSpace(in.Name) == "" {
		return abortf("主题名称必填")
	}
	if in.PriceFen < 0 {
		return abortf("价格不能为负")
	}
	if len(in.Prizes) == 0 {
		return abortf("奖品不能为空")
	}
	if in.ConfigJson != "" && !json.Valid([]byte(in.ConfigJson)) {
		return abortf("configJson 不是合法 JSON")
	}

	seen := map[int64]bool{}
	for _, p := range in.Prizes {
		if p == nil {
			return abortf("奖品不能为空")
		}
		if p.PrizeIndex < 0 || seen[p.PrizeIndex] {
			return abortf("prizeIndex 重复或非法")
		}
		seen[p.PrizeIndex] = true
		if strings.TrimSpace(p.Name) == "" {
			return abortf("奖品名必填")
		}
		if p.Weight < 0 || p.Quantity < -1 || p.Remaining < -1 {
			return abortf("权重/数量非法")
		}
		if p.SpecialKind != "" {
			kind := strings.ToUpper(p.SpecialKind)
			if !playPoolSpecialKindRe.MatchString(kind) {
				return abortf("specialKind 非法: %s", p.SpecialKind)
			}
			p.SpecialKind = kind
		}
		switch p.PrizeType {
		case 0, 1:
		case 2:
			if p.SkuId <= 0 {
				return abortf("实物奖 %s 缺少 skuId", p.Name)
			}
		case 3:
			if p.ChildPoolId <= 0 {
				return abortf("箱子奖 %s 缺少 childPoolId", p.Name)
			}
		default:
			return abortf("prizeType 非法: %d", p.PrizeType)
		}
		if p.ExtraJson != "" && !json.Valid([]byte(p.ExtraJson)) {
			return abortf("奖品 %s 的 extraJson 不是合法 JSON", p.Name)
		}
	}

	switch in.Mode {
	case "gashapon":
		return validateGashaponPool(in)
	case "fullcar":
		return validateFullCarPool(in)
	case "combochain":
		return validateComboChainPool(in)
	case "dig":
		return validateDigPool(in)
	case "groupbuy":
		return validateGroupBuyPool(in)
	case "memorymatch":
		return validateMemoryMatchPool(in)
	}
	return nil
}

// modelPlayPoolPrize 把 proto 草稿行转成落库行（草稿写入/投影共用）。
func modelPlayPoolPrize(p *play.PlayPoolPrize) gamemodelnc.PlayPoolPrize {
	remaining := p.Remaining
	if remaining == 0 && p.Quantity != 0 {
		remaining = p.Quantity
	}
	prizeType := p.PrizeType
	if prizeType == 0 {
		prizeType = 1
	}
	isPrize := int32(0)
	if p.IsPrize {
		isPrize = 1
	}
	parentIndex := int32(-1)
	if p.HasParent {
		parentIndex = int32(p.ParentIndex)
	}
	extra := strings.TrimSpace(p.ExtraJson)
	if extra == "" {
		extra = "{}"
	}
	return gamemodelnc.PlayPoolPrize{
		PrizeIndex:       int32(p.PrizeIndex),
		PrizeGrade:       p.PrizeGrade,
		SpecialKind:      strings.ToUpper(p.SpecialKind),
		PrizeType:        int32(prizeType),
		Name:             p.Name,
		SkuId:            p.SkuId,
		ChildPoolId:      p.ChildPoolId,
		Weight:           p.Weight,
		Quantity:         int32(p.Quantity),
		Remaining:        int32(remaining),
		IsPrize:          isPrize,
		TargetValue:      int32(p.TargetValue),
		CashFen:          p.CashFen,
		PointsAmount:     p.PointsAmount,
		CouponTemplateNo: p.CouponTemplateNo,
		CostFen:          p.CostFen,
		ImageUrl:         p.ImageUrl,
		ParentIndex:      parentIndex,
		ExtraJson:        extra,
	}
}

func validateGashaponPool(in *play.CreatePlayPoolDraftReq) error {
	specials := 0
	for _, p := range in.Prizes {
		if p.SpecialKind == "" {
			if p.Quantity < 0 {
				return abortf("扭蛋普通赏 %s 必须限量", p.Name)
			}
			continue
		}
		switch p.SpecialKind {
		case "LAST", "MVP", "LUCKY":
		default:
			return abortf("扭蛋不支持 specialKind=%s", p.SpecialKind)
		}
		if p.CashFen <= 0 && p.PointsAmount <= 0 && p.CouponTemplateNo == "" {
			return abortf("特别赏 %s 缺少奖金配置", p.Name)
		}
		specials++
	}
	if specials == 0 {
		return abortf("扭蛋必须配置至少一个特别赏")
	}
	return nil
}

func validateFullCarPool(in *play.CreatePlayPoolDraftReq) error {
	if _, err := parseFullCarPoolConfig(in.ConfigJson); err != nil {
		return abortf("整车 configJson 非法: %v", err)
	}
	settlements, entries := 0, 0
	for _, p := range in.Prizes {
		switch p.SpecialKind {
		case "":
			entries++
		case "SETTLEMENT":
			if p.TargetValue != 1 && p.TargetValue != 2 {
				return abortf("整车结算奖 %s 的 targetValue 必须是 1(幸运) 或 2(普通)", p.Name)
			}
			if p.Weight <= 0 {
				return abortf("整车结算奖 %s 权重必须大于 0", p.Name)
			}
			if p.CashFen <= 0 && p.PointsAmount <= 0 && p.CouponTemplateNo == "" {
				return abortf("整车结算奖 %s 缺少奖金配置", p.Name)
			}
			settlements++
		default:
			return abortf("整车不支持 specialKind=%s", p.SpecialKind)
		}
	}
	if entries == 0 {
		return abortf("整车至少需要一个普通条目")
	}
	if settlements == 0 {
		return abortf("整车至少需要一个 SETTLEMENT 结算奖")
	}
	return nil
}

func validateComboChainPool(in *play.CreatePlayPoolDraftReq) error {
	if _, err := parseComboChainPoolConfig(in.ConfigJson); err != nil {
		return abortf("连击链 configJson 非法: %v", err)
	}
	globals := 0
	for _, p := range in.Prizes {
		switch p.SpecialKind {
		case "GLOBAL":
			if p.TargetValue < 0 {
				return abortf("全局奖 %s 的 targetValue(槽位) 不能为负", p.Name)
			}
			if p.CashFen <= 0 && p.PointsAmount <= 0 && p.CouponTemplateNo == "" {
				return abortf("全局奖 %s 缺少奖金配置", p.Name)
			}
			globals++
		case "COMBO_TIER", "EMPTY_STREAK", "WIN_STREAK":
			if p.CashFen <= 0 && p.PointsAmount <= 0 && p.CouponTemplateNo == "" {
				return abortf("共享奖 %s 缺少奖金配置", p.Name)
			}
			if p.Quantity < 0 {
				return abortf("共享奖 %s 必须配置有限数量", p.Name)
			}
			if p.ExtraJson != "" {
				var extra struct {
					TierWeight           int32 `json:"tierWeight"`
					ConcentrateThreshold int32 `json:"concentrateThreshold"`
					StreakThreshold      int32 `json:"streakThreshold"`
				}
				if err := json.Unmarshal([]byte(p.ExtraJson), &extra); err != nil {
					return abortf("共享奖 %s 的 extraJson 非法: %v", p.Name, err)
				}
			}
		default:
			return abortf("连击链不支持 specialKind=%s", p.SpecialKind)
		}
	}
	if globals == 0 {
		return abortf("连击链至少需要一个 GLOBAL 全局奖")
	}
	return nil
}

func validateDigPool(in *play.CreatePlayPoolDraftReq) error {
	cfg, err := parseDigPoolConfig(in.ConfigJson)
	if err != nil {
		return abortf("挖宝 configJson 非法: %v", err)
	}
	cells := map[int64]bool{}
	for _, p := range in.Prizes {
		if p.SpecialKind != "" {
			return abortf("挖宝不支持 specialKind=%s", p.SpecialKind)
		}
		if p.TargetValue < 0 || p.TargetValue >= cfg.CellCount {
			return abortf("挖宝格 %s 的 targetValue 必须在 0..%d", p.Name, cfg.CellCount-1)
		}
		if cells[p.TargetValue] {
			return abortf("挖宝格 targetValue 重复: %d", p.TargetValue)
		}
		cells[p.TargetValue] = true
		if p.PrizeType == 2 && (p.CashFen > 0 || p.PointsAmount > 0 || p.CouponTemplateNo != "") {
			return abortf("挖宝实物格 %s 不能同时配置现金/积分/券", p.Name)
		}
	}
	return nil
}

func validateGroupBuyPool(in *play.CreatePlayPoolDraftReq) error {
	if _, err := parseGroupBuyPoolConfig(in.ConfigJson); err != nil {
		return abortf("拼团 configJson 非法: %v", err)
	}
	luckyIndexes := map[int64]bool{}
	for _, p := range in.Prizes {
		if p.SpecialKind == "LUCKY" {
			if p.Quantity < 0 {
				return abortf("幸运奖 %s 必须配置有限数量", p.Name)
			}
			if p.CashFen <= 0 && p.PointsAmount <= 0 && p.CouponTemplateNo == "" {
				return abortf("幸运奖 %s 缺少奖金配置", p.Name)
			}
			luckyIndexes[p.PrizeIndex] = true
		}
	}
	if len(luckyIndexes) == 0 {
		return abortf("拼团至少需要一个 LUCKY 幸运奖")
	}
	ordinary := map[int64]bool{}
	for _, p := range in.Prizes {
		if p.SpecialKind == "" && !p.HasParent {
			ordinary[p.PrizeIndex] = true
		}
	}
	if len(ordinary) == 0 {
		return abortf("拼团至少需要一个普通条目")
	}
	for _, p := range in.Prizes {
		switch p.SpecialKind {
		case "":
			if p.HasParent {
				if !ordinary[p.ParentIndex] {
					return abortf("拼团子奖 %s 指向的父条目不存在", p.Name)
				}
				if p.CashFen <= 0 && p.PointsAmount <= 0 && p.CouponTemplateNo == "" {
					return abortf("拼团子奖 %s 缺少奖金配置", p.Name)
				}
			}
		case "LUCKY":
			if p.HasParent {
				return abortf("奖品 %s 只有普通子奖才能引用父奖", p.Name)
			}
		case "LUCKY_CHILD":
			if !luckyIndexes[p.TargetValue] {
				return abortf("幸运子奖 %s 指向的幸运奖不存在", p.Name)
			}
			if p.CashFen <= 0 && p.PointsAmount <= 0 && p.CouponTemplateNo == "" {
				return abortf("幸运子奖 %s 缺少奖金配置", p.Name)
			}
		default:
			return abortf("拼团不支持 specialKind=%s", p.SpecialKind)
		}
	}
	return nil
}

func validateMemoryMatchPool(in *play.CreatePlayPoolDraftReq) error {
	cfg, err := parseMemoryMatchPoolConfig(in.ConfigJson)
	if err != nil {
		return abortf("记忆翻牌 configJson 非法: %v", err)
	}
	pairs := map[int64]bool{}
	for _, p := range in.Prizes {
		if p.SpecialKind != "" {
			return abortf("记忆翻牌不支持 specialKind=%s", p.SpecialKind)
		}
		if p.TargetValue < 0 || p.TargetValue > cfg.MaxPairCount {
			return abortf("记忆翻牌奖 %s 的 targetValue 必须在 0..%d", p.Name, cfg.MaxPairCount)
		}
		if pairs[p.TargetValue] {
			return abortf("记忆翻牌 targetValue 重复: %d", p.TargetValue)
		}
		pairs[p.TargetValue] = true
		if p.Weight <= 0 {
			return abortf("记忆翻牌奖 %s 权重必须大于 0", p.Name)
		}
	}
	return nil
}
