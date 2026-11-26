package logic

import (
	"context"
	"time"

	"gacha_srv/app/ops/internal/report"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBusinessDailyReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBusinessDailyReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBusinessDailyReportLogic {
	return &GetBusinessDailyReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetBusinessDailyReport 经营日报：只读 gacha_report 读模型。
func (l *GetBusinessDailyReportLogic) GetBusinessDailyReport(in *ops.BusinessDailyReportRequest) (*ops.BusinessDailyReportReply, error) {
	loc := time.FixedZone("CST", 8*3600)
	end := time.Now().In(loc)
	start := end.AddDate(0, 0, -29)
	var err error
	if in.GetStartDate() != "" {
		start, err = time.ParseInLocation("2006-01-02", in.GetStartDate(), loc)
		if err != nil {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "startDate 格式应为 YYYY-MM-DD")
		}
	}
	if in.GetEndDate() != "" {
		end, err = time.ParseInLocation("2006-01-02", in.GetEndDate(), loc)
		if err != nil {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "endDate 格式应为 YYYY-MM-DD")
		}
	}
	if end.Before(start) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "开始日期不能晚于结束日期")
	}
	if end.Sub(start) > 366*24*time.Hour {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "查询区间不能超过 366 天")
	}

	result, err := report.QueryDaily(l.svcCtx.ReportDB, start, end)
	if err != nil {
		l.Errorf("GetBusinessDailyReport failed, start: %s, end: %s, err: %v", start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "经营日报查询失败")
	}
	reply := &ops.BusinessDailyReportReply{
		Summary: summaryToProto(result.Summary),
		Month:   summaryToProto(result.Month),
		Rows:    make([]*ops.BusinessDailyRow, 0, len(result.Rows)),
	}
	for _, row := range result.Rows {
		reply.Rows = append(reply.Rows, &ops.BusinessDailyRow{
			Date:                      row.Date,
			Registrations:             row.Registrations,
			ActiveUsers:               row.ActiveUsers,
			PayingUsers:               row.PayingUsers,
			RefundableFen:             row.RefundableFen,
			BalanceFen:                row.BalanceFen,
			InventoryValueFen:         row.InventoryValueFen,
			DrawCount:                 row.DrawCount,
			DrawAmountFen:             row.DrawAmountFen,
			PrizeAmountFen:            row.PrizeAmountFen,
			GrossProfitFen:            row.GrossProfitFen,
			GrossMarginBps:            row.GrossMarginBps,
			TotalReceivedFen:          row.TotalReceivedFen,
			TotalRechargeFen:          row.TotalRechargeFen,
			TotalRefundFen:            row.TotalRefundFen,
			AlipayRechargeFen:         row.AlipayRechargeFen,
			HuifuRechargeFen:          row.HuifuRechargeFen,
			AlipayRefundFen:           row.AlipayRefundFen,
			HuifuRefundFen:            row.HuifuRefundFen,
			RechargeOriginalRefundFen: row.RechargeOriginalRefundFen,
			UserRefundFen:             row.UserRefundFen,
			RechargeMinusRefundFen:    row.RechargeMinusRefundFen,
			RefundRateBps:             row.RefundRateBps,
		})
	}
	return reply, nil
}

func summaryToProto(s report.Summary) *ops.BusinessDailySummary {
	return &ops.BusinessDailySummary{
		Registrations:             s.Registrations,
		PayingUsers:               s.PayingUsers,
		DrawCount:                 s.DrawCount,
		DrawAmountFen:             s.DrawAmountFen,
		PrizeAmountFen:            s.PrizeAmountFen,
		GrossProfitFen:            s.GrossProfitFen,
		GrossMarginBps:            s.GrossMarginBps,
		TotalReceivedFen:          s.TotalReceivedFen,
		TotalRechargeFen:          s.TotalRechargeFen,
		TotalRefundFen:            s.TotalRefundFen,
		AlipayRechargeFen:         s.AlipayRechargeFen,
		HuifuRechargeFen:          s.HuifuRechargeFen,
		AlipayRefundFen:           s.AlipayRefundFen,
		HuifuRefundFen:            s.HuifuRefundFen,
		RechargeOriginalRefundFen: s.RechargeOriginalRefundFen,
		UserRefundFen:             s.UserRefundFen,
		RechargeMinusRefundFen:    s.RechargeMinusRefundFen,
		RefundRateBps:             s.RefundRateBps,
	}
}
