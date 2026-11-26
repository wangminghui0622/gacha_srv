package reporting

import (
	"context"
	"time"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetentionReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRetentionReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetentionReportLogic {
	return &RetentionReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

type RetentionCell struct {
	DayNo         int   `json:"dayNo"`
	TotalUsers    int64 `json:"totalUsers"`
	RetainedUsers int64 `json:"retainedUsers"`
	RateBps       int64 `json:"rateBps"`
	Available     bool  `json:"available"`
}

type RetentionRow struct {
	CohortDate string          `json:"cohortDate"`
	TotalUsers int64           `json:"totalUsers"`
	Cells      []RetentionCell `json:"cells"`
}

type RetentionReportResponse struct {
	Type   string         `json:"type"`
	DayNos []int          `json:"dayNos"`
	Rows   []RetentionRow `json:"rows"`
	Total  RetentionRow   `json:"total"`
}

// Query 留存报表：网关只做参数转发，聚合与读模型在 ops-rpc。
func (l *RetentionReportLogic) Query(reportType string, start, end time.Time) (*RetentionReportResponse, error) {
	reply, err := l.svcCtx.OpsRpc.GetRetentionReport(l.ctx, &ops.RetentionReportRequest{
		ReportType: reportType,
		StartDate:  start.Format("2006-01-02"),
		EndDate:    end.Format("2006-01-02"),
	})
	if err != nil {
		l.Errorf("RetentionReport failed, type: %s, start: %s, end: %s, err: %v",
			reportType, start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.FromError(err)
	}

	out := &RetentionReportResponse{
		Type:   reply.GetReportType(),
		DayNos: make([]int, 0, len(reply.GetDayNos())),
		Rows:   make([]RetentionRow, 0, len(reply.GetRows())),
	}
	for _, dayNo := range reply.GetDayNos() {
		out.DayNos = append(out.DayNos, int(dayNo))
	}
	for _, row := range reply.GetRows() {
		out.Rows = append(out.Rows, retentionRowFromProto(row))
	}
	out.Total = retentionRowFromProto(reply.GetTotal())
	return out, nil
}

func retentionRowFromProto(row *ops.RetentionRow) RetentionRow {
	if row == nil {
		return RetentionRow{Cells: []RetentionCell{}}
	}
	out := RetentionRow{
		CohortDate: row.GetCohortDate(),
		TotalUsers: row.GetTotalUsers(),
		Cells:      make([]RetentionCell, 0, len(row.GetCells())),
	}
	for _, cell := range row.GetCells() {
		out.Cells = append(out.Cells, RetentionCell{
			DayNo:         int(cell.GetDayNo()),
			TotalUsers:    cell.GetTotalUsers(),
			RetainedUsers: cell.GetRetainedUsers(),
			RateBps:       cell.GetRateBps(),
			Available:     cell.GetAvailable(),
		})
	}
	return out
}
