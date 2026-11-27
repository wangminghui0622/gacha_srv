package gameimpl

import (
	"testing"

	"gacha_srv/app/play/play"
)

func gashaponDraftWith(prizes ...*play.PlayPoolPrize) *play.CreatePlayPoolDraftReq {
	return &play.CreatePlayPoolDraftReq{
		Mode:   "gashapon",
		Name:   "特别赏校验",
		Prizes: prizes,
	}
}

func TestValidateGashaponPoolAllowsGoodsSpecial(t *testing.T) {
	in := gashaponDraftWith(
		&play.PlayPoolPrize{PrizeIndex: 0, PrizeType: 1, Name: "普通赏", Quantity: 5},
		&play.PlayPoolPrize{PrizeIndex: 1, PrizeType: 2, Name: "Last赏", SpecialKind: "LAST", SkuId: 9007, Quantity: 1},
		&play.PlayPoolPrize{PrizeIndex: 2, PrizeType: 1, Name: "幸运赏", SpecialKind: "LUCKY", PointsAmount: 100, Quantity: 1},
	)
	if err := validatePlayPoolDraft(in); err != nil {
		t.Fatalf("goods special should be allowed: %v", err)
	}
}

func TestValidateGashaponPoolRejectsGoodsSpecialWithVirtualAssets(t *testing.T) {
	in := gashaponDraftWith(
		&play.PlayPoolPrize{PrizeIndex: 0, PrizeType: 1, Name: "普通赏", Quantity: 5},
		&play.PlayPoolPrize{PrizeIndex: 1, PrizeType: 2, Name: "Last赏", SpecialKind: "LAST", SkuId: 9007, CashFen: 500, Quantity: 1},
	)
	if err := validatePlayPoolDraft(in); err == nil {
		t.Fatal("goods special with cash should be rejected")
	}
}
