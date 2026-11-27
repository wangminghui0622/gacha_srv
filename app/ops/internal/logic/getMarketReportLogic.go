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

type GetMarketReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMarketReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMarketReportLogic {
	return &GetMarketReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMarketReport 市场报表：读 gacha_report.market_daily，缺失日期补 0。
func (l *GetMarketReportLogic) GetMarketReport(in *ops.MarketReportRequest) (*ops.MarketReportReply, error) {
	loc := time.FixedZone("CST", 8*3600)
	end := time.Now().In(loc).AddDate(0, 0, -1)
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

	result, err := report.QueryMarket(l.svcCtx.ReportDB, start, end)
	if err != nil {
		l.Errorf("GetMarketReport failed, start: %s, end: %s, err: %v",
			start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "市场报表查询失败")
	}
	reply := &ops.MarketReportReply{Rows: make([]*ops.MarketReportRow, 0, len(result.Rows))}
	for _, row := range result.Rows {
		reply.Rows = append(reply.Rows, &ops.MarketReportRow{
			Date:            row.Date,
			Participants:    row.Participants,
			NewOrders:       row.NewOrders,
			AmountTotalFen:  row.AmountTotalFen,
			AmountNormalFen: row.AmountNormalFen,
			AmountSampleFen: row.AmountSampleFen,
		})
	}
	return reply, nil
}
