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

type GetRevenueReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRevenueReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRevenueReportLogic {
	return &GetRevenueReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetRevenueReport 营收报表：只读 gacha_report.revenue_daily。
func (l *GetRevenueReportLogic) GetRevenueReport(in *ops.RevenueReportRequest) (*ops.RevenueReportReply, error) {
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

	result, err := report.QueryRevenue(l.svcCtx.ReportDB, start, end)
	if err != nil {
		l.Errorf("GetRevenueReport failed, start: %s, end: %s, err: %v",
			start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "营收报表查询失败")
	}

	reply := &ops.RevenueReportReply{Rows: make([]*ops.RevenueReportRow, 0, len(result.Rows))}
	for _, row := range result.Rows {
		reply.Rows = append(reply.Rows, &ops.RevenueReportRow{
			Date:            row.Date,
			Registrations:   row.Registrations,
			ActiveUsers:     row.ActiveUsers,
			NewActiveUsers:  row.NewActiveUsers,
			OldActiveUsers:  row.OldActiveUsers,
			ActiveUsers_7D:  row.ActiveUsers7d,
			ActiveUsers_15D: row.ActiveUsers15d,
			Churn_1DBps:     row.Churn1dBps,
			Churn_3DBps:     row.Churn3dBps,
			PayAmountFen:    row.PayAmountFen,
			NewPayAmountFen: row.NewPayAmountFen,
			OldPayAmountFen: row.OldPayAmountFen,
			RefundAmountFen: row.RefundAmountFen,
			RefundUsers:     row.RefundUsers,
			PayingUsers:     row.PayingUsers,
			NewPayingUsers:  row.NewPayingUsers,
			OldPayingUsers:  row.OldPayingUsers,
			ArpuFen:         row.ArpuFen,
			ArppuFen:        row.ArppuFen,
			PenetrationBps:  row.PenetrationBps,
			NetRevenueFen:   row.NetRevenueFen,
		})
	}
	return reply, nil
}
