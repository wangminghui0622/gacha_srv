package reporting

import (
	"context"
	"time"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BusinessDailyReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBusinessDailyReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BusinessDailyReportLogic {
	return &BusinessDailyReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

type DailyReportSummary struct {
	Registrations             int64 `json:"registrations"`
	PayingUsers               int64 `json:"payingUsers"`
	DrawCount                 int64 `json:"drawCount"`
	DrawAmountFen             int64 `json:"drawAmountFen"`
	PrizeAmountFen            int64 `json:"prizeAmountFen"`
	GrossProfitFen            int64 `json:"grossProfitFen"`
	GrossMarginBps            int64 `json:"grossMarginBps"`
	TotalReceivedFen          int64 `json:"totalReceivedFen"`
	TotalRechargeFen          int64 `json:"totalRechargeFen"`
	TotalRefundFen            int64 `json:"totalRefundFen"`
	AlipayRechargeFen         int64 `json:"alipayRechargeFen"`
	HuifuRechargeFen          int64 `json:"huifuRechargeFen"`
	AlipayRefundFen           int64 `json:"alipayRefundFen"`
	HuifuRefundFen            int64 `json:"huifuRefundFen"`
	RechargeOriginalRefundFen int64 `json:"rechargeOriginalRefundFen"`
	UserRefundFen             int64 `json:"userRefundFen"`
	RechargeMinusRefundFen    int64 `json:"rechargeMinusRefundFen"`
	RefundRateBps             int64 `json:"refundRateBps"`
}

type DailyReportRow struct {
	Date                      string `json:"date"`
	Registrations             int64  `json:"registrations"`
	ActiveUsers               int64  `json:"activeUsers"`
	PayingUsers               int64  `json:"payingUsers"`
	RefundableFen             int64  `json:"refundableFen"`
	BalanceFen                int64  `json:"balanceFen"`
	InventoryValueFen         int64  `json:"inventoryValueFen"`
	DrawCount                 int64  `json:"drawCount"`
	DrawAmountFen             int64  `json:"drawAmountFen"`
	PrizeAmountFen            int64  `json:"prizeAmountFen"`
	GrossProfitFen            int64  `json:"grossProfitFen"`
	GrossMarginBps            int64  `json:"grossMarginBps"`
	TotalReceivedFen          int64  `json:"totalReceivedFen"`
	TotalRechargeFen          int64  `json:"totalRechargeFen"`
	TotalRefundFen            int64  `json:"totalRefundFen"`
	AlipayRechargeFen         int64  `json:"alipayRechargeFen"`
	HuifuRechargeFen          int64  `json:"huifuRechargeFen"`
	AlipayRefundFen           int64  `json:"alipayRefundFen"`
	HuifuRefundFen            int64  `json:"huifuRefundFen"`
	RechargeOriginalRefundFen int64  `json:"rechargeOriginalRefundFen"`
	UserRefundFen             int64  `json:"userRefundFen"`
	RechargeMinusRefundFen    int64  `json:"rechargeMinusRefundFen"`
	RefundRateBps             int64  `json:"refundRateBps"`
}

type DailyReportResponse struct {
	Summary DailyReportSummary `json:"summary"`
	Month   DailyReportSummary `json:"month"`
	Rows    []DailyReportRow   `json:"rows"`
}

// Query 经营日报：网关只做参数转发，聚合与读模型在 ops-rpc。
func (l *BusinessDailyReportLogic) Query(start, end time.Time) (*DailyReportResponse, error) {
	reply, err := l.svcCtx.OpsRpc.GetBusinessDailyReport(l.ctx, &ops.BusinessDailyReportRequest{
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
	})
	if err != nil {
		l.Errorf("BusinessDailyReport failed, start: %s, end: %s, err: %v", start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.FromError(err)
	}
	out := &DailyReportResponse{
		Summary: summaryFromProto(reply.GetSummary()),
		Month:   summaryFromProto(reply.GetMonth()),
		Rows:    make([]DailyReportRow, 0, len(reply.GetRows())),
	}
	for _, row := range reply.GetRows() {
		out.Rows = append(out.Rows, DailyReportRow{
			Date:                      row.GetDate(),
			Registrations:             row.GetRegistrations(),
			ActiveUsers:               row.GetActiveUsers(),
			PayingUsers:               row.GetPayingUsers(),
			RefundableFen:             row.GetRefundableFen(),
			BalanceFen:                row.GetBalanceFen(),
			InventoryValueFen:         row.GetInventoryValueFen(),
			DrawCount:                 row.GetDrawCount(),
			DrawAmountFen:             row.GetDrawAmountFen(),
			PrizeAmountFen:            row.GetPrizeAmountFen(),
			GrossProfitFen:            row.GetGrossProfitFen(),
			GrossMarginBps:            row.GetGrossMarginBps(),
			TotalReceivedFen:          row.GetTotalReceivedFen(),
			TotalRechargeFen:          row.GetTotalRechargeFen(),
			TotalRefundFen:            row.GetTotalRefundFen(),
			AlipayRechargeFen:         row.GetAlipayRechargeFen(),
			HuifuRechargeFen:          row.GetHuifuRechargeFen(),
			AlipayRefundFen:           row.GetAlipayRefundFen(),
			HuifuRefundFen:            row.GetHuifuRefundFen(),
			RechargeOriginalRefundFen: row.GetRechargeOriginalRefundFen(),
			UserRefundFen:             row.GetUserRefundFen(),
			RechargeMinusRefundFen:    row.GetRechargeMinusRefundFen(),
			RefundRateBps:             row.GetRefundRateBps(),
		})
	}
	return out, nil
}

func summaryFromProto(s *ops.BusinessDailySummary) DailyReportSummary {
	if s == nil {
		return DailyReportSummary{}
	}
	return DailyReportSummary{
		Registrations:             s.GetRegistrations(),
		PayingUsers:               s.GetPayingUsers(),
		DrawCount:                 s.GetDrawCount(),
		DrawAmountFen:             s.GetDrawAmountFen(),
		PrizeAmountFen:            s.GetPrizeAmountFen(),
		GrossProfitFen:            s.GetGrossProfitFen(),
		GrossMarginBps:            s.GetGrossMarginBps(),
		TotalReceivedFen:          s.GetTotalReceivedFen(),
		TotalRechargeFen:          s.GetTotalRechargeFen(),
		TotalRefundFen:            s.GetTotalRefundFen(),
		AlipayRechargeFen:         s.GetAlipayRechargeFen(),
		HuifuRechargeFen:          s.GetHuifuRechargeFen(),
		AlipayRefundFen:           s.GetAlipayRefundFen(),
		HuifuRefundFen:            s.GetHuifuRefundFen(),
		RechargeOriginalRefundFen: s.GetRechargeOriginalRefundFen(),
		UserRefundFen:             s.GetUserRefundFen(),
		RechargeMinusRefundFen:    s.GetRechargeMinusRefundFen(),
		RefundRateBps:             s.GetRefundRateBps(),
	}
}
