package reporting

import (
	"context"
	"time"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarketReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarketReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarketReportLogic {
	return &MarketReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

type MarketReportRow struct {
	Date            string `json:"date"`
	Participants    int64  `json:"participants"`
	NewOrders       int64  `json:"newOrders"`
	AmountTotalFen  int64  `json:"amountTotalFen"`
	AmountNormalFen int64  `json:"amountNormalFen"`
	AmountSampleFen int64  `json:"amountSampleFen"`
}

type MarketReportResponse struct {
	Rows []MarketReportRow `json:"rows"`
}

// Query 市场报表：网关只做参数转发，读模型在 ops-rpc。
func (l *MarketReportLogic) Query(start, end time.Time) (*MarketReportResponse, error) {
	reply, err := l.svcCtx.OpsRpc.GetMarketReport(l.ctx, &ops.MarketReportRequest{
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
	})
	if err != nil {
		l.Errorf("MarketReport failed, start: %s, end: %s, err: %v",
			start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.FromError(err)
	}
	out := &MarketReportResponse{Rows: make([]MarketReportRow, 0, len(reply.GetRows()))}
	for _, row := range reply.GetRows() {
		out.Rows = append(out.Rows, MarketReportRow{
			Date:            row.GetDate(),
			Participants:    row.GetParticipants(),
			NewOrders:       row.GetNewOrders(),
			AmountTotalFen:  row.GetAmountTotalFen(),
			AmountNormalFen: row.GetAmountNormalFen(),
			AmountSampleFen: row.GetAmountSampleFen(),
		})
	}
	return out, nil
}
