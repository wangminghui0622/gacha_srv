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

type GetRealtimeReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRealtimeReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRealtimeReportLogic {
	return &GetRealtimeReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetRealtimeReport 实时报表：今天/昨天按 10 分钟桶查询业务库。
func (l *GetRealtimeReportLogic) GetRealtimeReport(in *ops.RealtimeReportRequest) (*ops.RealtimeReportReply, error) {
	result, err := report.QueryRealtime(l.svcCtx.DB, time.Now())
	if err != nil {
		l.Errorf("GetRealtimeReport failed, err: %v", err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "实时报表查询失败")
	}
	reply := &ops.RealtimeReportReply{
		Summary: &ops.RealtimeSummary{
			TotalRegistrations: result.Summary.TotalRegistrations,
			TotalPayingUsers:   result.Summary.TotalPayingUsers,
			TotalPayFen:        result.Summary.TotalPayFen,
			TotalRechargeUsers: result.Summary.TotalRechargeUsers,
			TotalRechargeFen:   result.Summary.TotalRechargeFen,
			TotalRefundFen:     result.Summary.TotalRefundFen,
			ArppuFen:           result.Summary.ArppuFen,
			TodayRegistrations: result.Summary.TodayRegistrations,
			TodayPayFen:        result.Summary.TodayPayFen,
			TodayRechargeFen:   result.Summary.TodayRechargeFen,
		},
		Labels: result.Labels,
		Charts: make([]*ops.RealtimeChart, 0, len(result.Charts)),
	}
	for _, chart := range result.Charts {
		item := &ops.RealtimeChart{
			Title:  chart.Title,
			Series: make([]*ops.RealtimeSeries, 0, len(chart.Series)),
		}
		for _, series := range chart.Series {
			item.Series = append(item.Series, &ops.RealtimeSeries{
				Name:   series.Name,
				Kind:   series.Kind,
				Values: series.Values,
			})
		}
		reply.Charts = append(reply.Charts, item)
	}
	return reply, nil
}
