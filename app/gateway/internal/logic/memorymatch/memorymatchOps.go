package memorymatch

import (
	"gacha_srv/app/gateway/internal/types"
	playClient "gacha_srv/app/play/playClient"
)

// prizeToView 奖品 pb → 网关视图
func prizeToView(p *playClient.PrizeItem) types.DigPrizeItem {
	if p == nil {
		return types.DigPrizeItem{}
	}
	return types.DigPrizeItem{
		Kind:             p.Kind,
		Amount:           p.Amount,
		Points:           p.Points,
		CouponTemplateNo: p.CouponTemplateNo,
		CouponName:       p.CouponName,
		SkuId:            p.SkuId,
		ItemName:         p.ItemName,
	}
}

// settlementToView 终局结算 pb → 网关视图
func settlementToView(s *playClient.MemoryMatchSettlementView) *types.MemoryMatchSettlementView {
	if s == nil {
		return nil
	}
	return &types.MemoryMatchSettlementView{
		TotalPairCount: s.TotalPairCount,
		FinalHash:      s.FinalHash,
		Prize:          prizeToView(s.Prize),
		Status:         s.Status,
	}
}

// appearanceToView 外观 pb → 网关视图
func appearanceToView(a *playClient.MemoryMatchAppearanceView) types.MemoryMatchAppearanceResp {
	if a == nil {
		return types.MemoryMatchAppearanceResp{OwnedFacePresetIds: []int64{}}
	}
	owned := a.OwnedFacePresetIds
	if owned == nil {
		owned = []int64{}
	}
	return types.MemoryMatchAppearanceResp{
		FacePresetId:       a.FacePresetId,
		BackPresetId:       a.BackPresetId,
		OwnedFacePresetIds: owned,
	}
}

// configToView 玩法配置 pb → 网关视图
func configToView(c *playClient.MemoryMatchConfigItem) types.MemoryMatchConfigDetail {
	d := types.MemoryMatchConfigDetail{
		ConfigId:              c.ConfigId,
		Name:                  c.Name,
		RoundsPerPurchase:     c.RoundsPerPurchase,
		MaxPairCount:          c.MaxPairCount,
		PurchasePrice:         c.PurchasePrice,
		WalletKind:            c.WalletKind,
		SecondDestinyEnabled:  c.SecondDestinyEnabled,
		DeckStrategy:          int64(c.DeckStrategy),
		PatternWeights:        c.PatternWeights,
		ExactMinPairCount:     c.ExactMinPairCount,
		ExactMaxPairCount:     c.ExactMaxPairCount,
		ExactAttemptLimit:     c.ExactAttemptLimit,
		DefaultFacePresetId:   c.DefaultFacePresetId,
		DefaultBackPresetId:   c.DefaultBackPresetId,
		ChampionMinTotalPairs: c.ChampionMinTotalPairs,
		ChampionShareBps:      c.ChampionShareBps,
		Status:                c.Status,
		FacePresets:           make([]types.MemoryMatchFacePreset, 0, len(c.FacePresets)),
		BackPresets:           make([]types.MemoryMatchBackPreset, 0, len(c.BackPresets)),
		Entries:               make([]types.MemoryMatchEntryItem, 0, len(c.Entries)),
	}
	for _, f := range c.FacePresets {
		fp := types.MemoryMatchFacePreset{PresetId: f.PresetId, Name: f.Name, Faces: make([]types.MemoryMatchFaceItem, 0, len(f.Faces))}
		for _, face := range f.Faces {
			fp.Faces = append(fp.Faces, types.MemoryMatchFaceItem{
				PatternId: face.PatternId, Name: face.Name, ImageUrl: face.ImageUrl,
			})
		}
		d.FacePresets = append(d.FacePresets, fp)
	}
	for _, b := range c.BackPresets {
		d.BackPresets = append(d.BackPresets, types.MemoryMatchBackPreset{
			PresetId:               b.PresetId,
			Name:                   b.Name,
			BackImageUrl:           b.BackImageUrl,
			FaceBackgroundImageUrl: b.FaceBackgroundImageUrl,
		})
	}
	for _, e := range c.Entries {
		d.Entries = append(d.Entries, types.MemoryMatchEntryItem{
			EntryId:        e.EntryId,
			Name:           e.Name,
			TotalPairCount: e.TotalPairCount,
			Prize:          prizeToView(e.Prize),
			Remaining:      e.Remaining,
		})
	}
	return d
}

// championToView 擂主 pb → 网关视图
func championToView(c *playClient.MemoryMatchChampionInfo) types.MemoryMatchChampionInfo {
	if c == nil {
		return types.MemoryMatchChampionInfo{}
	}
	return types.MemoryMatchChampionInfo{
		Exists:         c.Exists,
		UserId:         c.UserId,
		TotalPairCount: c.TotalPairCount,
		PendingIncome:  c.PendingIncome,
	}
}

// roundToView 回合 pb → 网关视图
func roundToView(r *playClient.GetMemoryMatchRoundResp) types.MemoryMatchRoundResp {
	hand := r.HandCardIds
	if hand == nil {
		hand = []int64{}
	}
	actions := r.Actions
	if actions == nil {
		actions = []string{}
	}
	return types.MemoryMatchRoundResp{
		RoundNo:                r.RoundNo,
		State:                  int64(r.State),
		HandCardIds:            hand,
		DrawPosition:           r.DrawPosition,
		PairCount:              r.PairCount,
		MaxPairCount:           r.MaxPairCount,
		AvailableDestiny:       r.AvailableDestiny,
		DestinyDraws:           r.DestinyDraws,
		SeedCommitment:         r.SeedCommitment,
		Actions:                actions,
		Terminal:               r.Terminal,
		ResultHash:             r.ResultHash,
		DestinyPatternId:       r.DestinyPatternId,
		SecondDestinyPatternId: r.SecondDestinyPatternId,
	}
}
