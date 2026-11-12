package saga

import (
	marketingClient "gacha_srv/app/marketing/marketingClient"
	moneyClient "gacha_srv/app/money/moneyClient"
	"gacha_srv/pkg/utils"
)

// WalletAuthorizeBranch 02 建单+冻结（资金域）：5 个玩法购买流共用。
func WalletAuthorizeBranch(moneyRpcDirect, gid string, userID, boxID int64, quote *marketingClient.QuoteCheckoutResp) Branch {
	return Branch{
		Action: moneyRpcDirect + "/money.Money/AuthorizeWalletPayment",
		Revert: moneyRpcDirect + "/money.Money/CancelPayment",
		Payload: &moneyClient.AuthorizeWalletPaymentReq{
			RequestId:          gid,
			UserId:             userID,
			BoxId:              boxID,
			GrossAmount:        utils.FenToYuan(quote.PayableFen),
			CashAmount:         utils.FenToYuan(quote.CashFen),
			LoyaltyAmount:      utils.FenToYuan(quote.LoyaltyFen),
			LoyaltyPointsDebit: quote.LoyaltyPointsDebit,
		},
	}
}

// WalletCaptureBranch 05 捕获核销（资金域末步）：5 个玩法购买流共用。
func WalletCaptureBranch(moneyRpcDirect, gid string, quote *marketingClient.QuoteCheckoutResp) Branch {
	return Branch{
		Action: moneyRpcDirect + "/money.Money/CaptureWalletPayment",
		Revert: moneyRpcDirect + "/money.Money/CaptureWalletPaymentRevert",
		Payload: &moneyClient.CaptureWalletPaymentReq{
			RequestId:  gid,
			QuoteToken: quote.QuoteToken,
		},
	}
}
