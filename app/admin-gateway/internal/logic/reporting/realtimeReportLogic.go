package reporting

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RealtimeReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRealtimeReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RealtimeReportLogic {
	return &RealtimeReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

type RealtimeSummary struct {
	TotalRegistrations int64 `json:"totalRegistrations"`
	TotalPayingUsers   int64 `json:"totalPayingUsers"`
	TotalPayFen        int64 `json:"totalPayFen"`
	TotalRechargeUsers int64 `json:"totalRechargeUsers"`
	TotalRechargeFen   int64 `json:"totalRechargeFen"`
	TotalRefundFen     int64 `json:"totalRefundFen"`
	ArppuFen           int64 `json:"arppuFen"`
	TodayRegistrations int64 `json:"todayRegistrations"`
	TodayPayFen        int64 `json:"todayPayFen"`
	TodayRechargeFen   int64 `json:"todayRechargeFen"`
}

type RealtimeSeries struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Values []int64 `json:"values"`
}

type RealtimeChart struct {
	Title  string           `json:"title"`
	Series []RealtimeSeries `json:"series"`
}

type RealtimeReportResponse struct {
	Summary RealtimeSummary `json:"summary"`
	Labels  []string        `json:"labels"`
	Charts  []RealtimeChart `json:"charts"`
}

// Query 实时报表：网关只做转发，今天/昨天数据由 ops-rpc 查询。
func (l *RealtimeReportLogic) Query() (*RealtimeReportResponse, error) {
	reply, err := l.svcCtx.OpsRpc.GetRealtimeReport(l.ctx, &ops.RealtimeReportRequest{})
	if err != nil {
		l.Errorf("RealtimeReport failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	summary := reply.GetSummary()
	out := &RealtimeReportResponse{
		Summary: RealtimeSummary{
			TotalRegistrations: summary.GetTotalRegistrations(),
			TotalPayingUsers:   summary.GetTotalPayingUsers(),
			TotalPayFen:        summary.GetTotalPayFen(),
			TotalRechargeUsers: summary.GetTotalRechargeUsers(),
			TotalRechargeFen:   summary.GetTotalRechargeFen(),
			TotalRefundFen:     summary.GetTotalRefundFen(),
			ArppuFen:           summary.GetArppuFen(),
			TodayRegistrations: summary.GetTodayRegistrations(),
			TodayPayFen:        summary.GetTodayPayFen(),
			TodayRechargeFen:   summary.GetTodayRechargeFen(),
		},
		Labels: reply.GetLabels(),
		Charts: make([]RealtimeChart, 0, len(reply.GetCharts())),
	}
	for _, chart := range reply.GetCharts() {
		item := RealtimeChart{
			Title:  chart.GetTitle(),
			Series: make([]RealtimeSeries, 0, len(chart.GetSeries())),
		}
		for _, series := range chart.GetSeries() {
			item.Series = append(item.Series, RealtimeSeries{
				Name:   series.GetName(),
				Kind:   series.GetKind(),
				Values: series.GetValues(),
			})
		}
		out.Charts = append(out.Charts, item)
	}
	return out, nil
}
