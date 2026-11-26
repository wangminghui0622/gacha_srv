package reporting

import (
	"context"
	"time"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RevenueReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevenueReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevenueReportLogic {
	return &RevenueReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

type RevenueReportRow struct {
	Date            string `json:"date"`
	Registrations   int64  `json:"registrations"`
	ActiveUsers     int64  `json:"activeUsers"`
	NewActiveUsers  int64  `json:"newActiveUsers"`
	OldActiveUsers  int64  `json:"oldActiveUsers"`
	ActiveUsers7d   int64  `json:"activeUsers7d"`
	ActiveUsers15d  int64  `json:"activeUsers15d"`
	Churn1dBps      int64  `json:"churn1dBps"`
	Churn3dBps      int64  `json:"churn3dBps"`
	PayAmountFen    int64  `json:"payAmountFen"`
	NewPayAmountFen int64  `json:"newPayAmountFen"`
	OldPayAmountFen int64  `json:"oldPayAmountFen"`
	RefundAmountFen int64  `json:"refundAmountFen"`
	RefundUsers     int64  `json:"refundUsers"`
	PayingUsers     int64  `json:"payingUsers"`
	NewPayingUsers  int64  `json:"newPayingUsers"`
	OldPayingUsers  int64  `json:"oldPayingUsers"`
	ArpuFen         int64  `json:"arpuFen"`
	ArppuFen        int64  `json:"arppuFen"`
	PenetrationBps  int64  `json:"penetrationBps"`
	NetRevenueFen   int64  `json:"netRevenueFen"`

	WechatPayFen           int64 `json:"wechatPayFen"`
	AlipayPayFen           int64 `json:"alipayPayFen"`
	HuifuPayFen            int64 `json:"huifuPayFen"`
	WechatGiftFen          int64 `json:"wechatGiftFen"`
	AlipayGiftFen          int64 `json:"alipayGiftFen"`
	HuifuGiftFen           int64 `json:"huifuGiftFen"`
	WechatRefundFen        int64 `json:"wechatRefundFen"`
	AlipayRefundFen        int64 `json:"alipayRefundFen"`
	HuifuRefundFen         int64 `json:"huifuRefundFen"`
	WechatGiftRefundFen    int64 `json:"wechatGiftRefundFen"`
	AlipayGiftRefundFen    int64 `json:"alipayGiftRefundFen"`
	HuifuGiftRefundFen     int64 `json:"huifuGiftRefundFen"`
	TaxFen                 int64 `json:"taxFen"`
	NewTaxFen              int64 `json:"newTaxFen"`
	OldTaxFen              int64 `json:"oldTaxFen"`
	WalletBalanceFen       int64 `json:"walletBalanceFen"`
	BonusBalanceFen        int64 `json:"bonusBalanceFen"`
	RedeemPointBalance     int64 `json:"redeemPointBalance"`
	LoyaltyPointBalance    int64 `json:"loyaltyPointBalance"`
	PlatformCurrencyPayFen int64 `json:"platformCurrencyPayFen"`
	ArpuNewFen             int64 `json:"arpuNewFen"`
	ArpuOldFen             int64 `json:"arpuOldFen"`
	PenetrationNewBps      int64 `json:"penetrationNewBps"`
	PenetrationOldBps      int64 `json:"penetrationOldBps"`
	RefundRateBps          int64 `json:"refundRateBps"`
	RevenueRateBps         int64 `json:"revenueRateBps"`
	RevenueArpuFen         int64 `json:"revenueArpuFen"`
}

type RevenueReportResponse struct {
	Rows []RevenueReportRow `json:"rows"`
}

// Query 营收报表：网关只做参数转发，聚合与读模型在 ops-rpc。
func (l *RevenueReportLogic) Query(start, end time.Time) (*RevenueReportResponse, error) {
	reply, err := l.svcCtx.OpsRpc.GetRevenueReport(l.ctx, &ops.RevenueReportRequest{
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
	})
	if err != nil {
		l.Errorf("RevenueReport failed, start: %s, end: %s, err: %v",
			start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.FromError(err)
	}
	out := &RevenueReportResponse{Rows: make([]RevenueReportRow, 0, len(reply.GetRows()))}
	for _, row := range reply.GetRows() {
		out.Rows = append(out.Rows, RevenueReportRow{
			Date:            row.GetDate(),
			Registrations:   row.GetRegistrations(),
			ActiveUsers:     row.GetActiveUsers(),
			NewActiveUsers:  row.GetNewActiveUsers(),
			OldActiveUsers:  row.GetOldActiveUsers(),
			ActiveUsers7d:   row.GetActiveUsers_7D(),
			ActiveUsers15d:  row.GetActiveUsers_15D(),
			Churn1dBps:      row.GetChurn_1DBps(),
			Churn3dBps:      row.GetChurn_3DBps(),
			PayAmountFen:    row.GetPayAmountFen(),
			NewPayAmountFen: row.GetNewPayAmountFen(),
			OldPayAmountFen: row.GetOldPayAmountFen(),
			RefundAmountFen: row.GetRefundAmountFen(),
			RefundUsers:     row.GetRefundUsers(),
			PayingUsers:     row.GetPayingUsers(),
			NewPayingUsers:  row.GetNewPayingUsers(),
			OldPayingUsers:  row.GetOldPayingUsers(),
			ArpuFen:         row.GetArpuFen(),
			ArppuFen:        row.GetArppuFen(),
			PenetrationBps:  row.GetPenetrationBps(),
			NetRevenueFen:   row.GetNetRevenueFen(),

			WechatPayFen:           row.GetWechatPayFen(),
			AlipayPayFen:           row.GetAlipayPayFen(),
			HuifuPayFen:            row.GetHuifuPayFen(),
			WechatGiftFen:          row.GetWechatGiftFen(),
			AlipayGiftFen:          row.GetAlipayGiftFen(),
			HuifuGiftFen:           row.GetHuifuGiftFen(),
			WechatRefundFen:        row.GetWechatRefundFen(),
			AlipayRefundFen:        row.GetAlipayRefundFen(),
			HuifuRefundFen:         row.GetHuifuRefundFen(),
			WechatGiftRefundFen:    row.GetWechatGiftRefundFen(),
			AlipayGiftRefundFen:    row.GetAlipayGiftRefundFen(),
			HuifuGiftRefundFen:     row.GetHuifuGiftRefundFen(),
			TaxFen:                 row.GetTaxFen(),
			NewTaxFen:              row.GetNewTaxFen(),
			OldTaxFen:              row.GetOldTaxFen(),
			WalletBalanceFen:       row.GetWalletBalanceFen(),
			BonusBalanceFen:        row.GetBonusBalanceFen(),
			RedeemPointBalance:     row.GetRedeemPointBalance(),
			LoyaltyPointBalance:    row.GetLoyaltyPointBalance(),
			PlatformCurrencyPayFen: row.GetPlatformCurrencyPayFen(),
			ArpuNewFen:             row.GetArpuNewFen(),
			ArpuOldFen:             row.GetArpuOldFen(),
			PenetrationNewBps:      row.GetPenetrationNewBps(),
			PenetrationOldBps:      row.GetPenetrationOldBps(),
			RefundRateBps:          row.GetRefundRateBps(),
			RevenueRateBps:         row.GetRevenueRateBps(),
			RevenueArpuFen:         row.GetRevenueArpuFen(),
		})
	}
	return out, nil
}
