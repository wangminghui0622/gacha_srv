package saga

import (
	"testing"

	marketingClient "gacha_srv/app/marketing/marketingClient"
	moneyClient "gacha_srv/app/money/moneyClient"

	"google.golang.org/protobuf/types/known/emptypb"
)

// TestPlayBranchesFixedOrder 锁定 5 分支顺序：lock → authorize → settle → grant → capture。
// 顺序是 SAGA 语义不变量：奖品先于捕获、锁先于扣款；补偿由 DTM 反向执行。
func TestPlayBranchesFixedOrder(t *testing.T) {
	quote := &marketingClient.QuoteCheckoutResp{
		QuoteToken:         "qt-1",
		PayableFen:         100,
		CashFen:            80,
		LoyaltyFen:         20,
		LoyaltyPointsDebit: 2,
	}
	got := PlayBranches("money-direct", "gid-1", 7, 9, quote, PlayBranchSet{
		LockPath:         "play/Lock",
		ReleasePath:      "play/Release",
		LockPayload:      &emptypb.Empty{},
		SettlePath:       "play/Settle",
		SettleRevertPath: "play/SettleRevert",
		SettlePayload:    &emptypb.Empty{},
		GrantPath:        "play/Grant",
		GrantRevertPath:  "play/GrantRevert",
		GrantPayload:     &emptypb.Empty{},
	})

	wantAction := []string{
		"play/Lock",
		"money-direct/money.Money/AuthorizeWalletPayment",
		"play/Settle",
		"play/Grant",
		"money-direct/money.Money/CaptureWalletPayment",
	}
	wantRevert := []string{
		"play/Release",
		"money-direct/money.Money/CancelPayment",
		"play/SettleRevert",
		"play/GrantRevert",
		"money-direct/money.Money/CaptureWalletPaymentRevert",
	}
	if len(got) != len(wantAction) {
		t.Fatalf("branch count = %d, want %d", len(got), len(wantAction))
	}
	for i := range wantAction {
		if got[i].Action != wantAction[i] {
			t.Fatalf("action[%d] = %q, want %q", i, got[i].Action, wantAction[i])
		}
		if got[i].Revert != wantRevert[i] {
			t.Fatalf("revert[%d] = %q, want %q", i, got[i].Revert, wantRevert[i])
		}
		if got[i].Payload == nil {
			t.Fatalf("payload[%d] is nil", i)
		}
	}

	// 02 冻结：金额/用户/箱子/忠诚度抵扣都要从 quote 正确映射
	auth, ok := got[1].Payload.(*moneyClient.AuthorizeWalletPaymentReq)
	if !ok {
		t.Fatalf("authorize payload type = %T", got[1].Payload)
	}
	if auth.RequestId != "gid-1" || auth.UserId != 7 || auth.BoxId != 9 {
		t.Fatalf("authorize identity = %+v", auth)
	}
	if auth.GrossAmount != "1.00" || auth.CashAmount != "0.80" || auth.LoyaltyAmount != "0.20" || auth.LoyaltyPointsDebit != 2 {
		t.Fatalf("authorize amounts = %+v", auth)
	}

	// 05 捕获：必须带同一报价 token
	capture, ok := got[4].Payload.(*moneyClient.CaptureWalletPaymentReq)
	if !ok {
		t.Fatalf("capture payload type = %T", got[4].Payload)
	}
	if capture.RequestId != "gid-1" || capture.QuoteToken != "qt-1" {
		t.Fatalf("capture = %+v", capture)
	}
}
