package gameimpl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
)

// ══════════════ 统一 play 赏池/奖品：共享工具 ═══════════════

func hashPlayPoolDraft(in *play.CreatePlayPoolDraftReq) string {
	payload := struct {
		Mode       string                `json:"mode"`
		Name       string                `json:"name"`
		PriceFen   int64                 `json:"priceFen"`
		ConfigJson string                `json:"configJson"`
		Prizes     []*play.PlayPoolPrize `json:"prizes"`
		IPID       uint64                `json:"ipId"`
		Tags       []string              `json:"tags"`
		Periods    int64                 `json:"periods"`
		Weight     int64                 `json:"weight"`
	}{in.Mode, in.Name, in.PriceFen, in.ConfigJson, in.Prizes, in.IpId, in.Tags, in.Periods, in.Weight}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func playPoolPrizesToProto(rows []gamemodelnc.PlayPoolPrize) []*play.PlayPoolPrize {
	out := make([]*play.PlayPoolPrize, 0, len(rows))
	for _, p := range rows {
		out = append(out, &play.PlayPoolPrize{
			PrizeIndex:       int64(p.PrizeIndex),
			PrizeGrade:       p.PrizeGrade,
			SpecialKind:      p.SpecialKind,
			PrizeType:        int64(p.PrizeType),
			Name:             p.Name,
			SkuId:            p.SkuId,
			ChildPoolId:      p.ChildPoolId,
			Weight:           p.Weight,
			Quantity:         int64(p.Quantity),
			Remaining:        int64(p.Remaining),
			IsPrize:          p.IsPrize == 1,
			TargetValue:      int64(p.TargetValue),
			CashFen:          p.CashFen,
			PointsAmount:     p.PointsAmount,
			CouponTemplateNo: p.CouponTemplateNo,
			CostFen:          p.CostFen,
			ImageUrl:         p.ImageUrl,
			ParentIndex:      int64(p.ParentIndex),
			ExtraJson:        p.ExtraJson,
			HasParent:        p.ParentIndex >= 0,
		})
	}
	return out
}
