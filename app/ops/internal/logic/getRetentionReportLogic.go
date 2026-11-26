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

type GetRetentionReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRetentionReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRetentionReportLogic {
	return &GetRetentionReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetRetentionReport 留存报表：只读 gacha_report 的 5 张留存读模型表。
func (l *GetRetentionReportLogic) GetRetentionReport(in *ops.RetentionReportRequest) (*ops.RetentionReportReply, error) {
	typ := in.GetReportType()
	if typ == "" {
		typ = report.RetentionTypeNew
	}
	if !report.IsRetentionType(typ) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "reportType 仅支持 new/active/new_pay/join/pay")
	}

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

	result, err := report.QueryRetention(l.svcCtx.ReportDB, typ, start, end)
	if err != nil {
		l.Errorf("GetRetentionReport failed, type: %s, start: %s, end: %s, err: %v",
			typ, start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "留存报表查询失败")
	}

	reply := &ops.RetentionReportReply{
		ReportType: result.Type,
		DayNos:     make([]int32, 0, len(result.DayNos)),
		Rows:       make([]*ops.RetentionRow, 0, len(result.Rows)),
	}
	for _, dayNo := range result.DayNos {
		reply.DayNos = append(reply.DayNos, int32(dayNo))
	}
	for _, row := range result.Rows {
		reply.Rows = append(reply.Rows, retentionRowToProto(row))
	}
	reply.Total = retentionRowToProto(result.Total)
	return reply, nil
}

func retentionRowToProto(row report.RetentionRow) *ops.RetentionRow {
	out := &ops.RetentionRow{
		CohortDate: row.CohortDate,
		TotalUsers: row.TotalUsers,
		Cells:      make([]*ops.RetentionCell, 0, len(row.Cells)),
	}
	for _, cell := range row.Cells {
		out.Cells = append(out.Cells, &ops.RetentionCell{
			DayNo:         int32(cell.DayNo),
			TotalUsers:    cell.TotalUsers,
			RetainedUsers: cell.RetainedUsers,
			RateBps:       cell.RateBps,
			Available:     cell.Available,
		})
	}
	return out
}
