package gameimpl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"gacha_srv/app/play/play"
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
		AdminJSON  string                `json:"adminJson"`
	}{in.Mode, in.Name, in.PriceFen, in.ConfigJson, in.Prizes, in.IpId, in.Tags, in.Periods, in.Weight, in.AdminJson}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
