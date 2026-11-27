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

type GetOrderReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetOrderReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderReportLogic {
	return &GetOrderReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetOrderReport 订单报表：按日期+用户聚合抽赏订单成本价。
func (l *GetOrderReportLogic) GetOrderReport(in *ops.OrderReportRequest) (*ops.OrderReportReply, error) {
	loc := time.FixedZone("CST", 8*3600)
	end := time.Now().In(loc)
	start := end.AddDate(0, 0, -31)
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

	result, err := report.QueryOrder(l.svcCtx.DB, start, end, in.GetUserId(), in.GetNickname(), in.GetPhoneUserId())
	if err != nil {
		l.Errorf("GetOrderReport failed, start: %s, end: %s, err: %v",
			start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "订单报表查询失败")
	}
	reply := &ops.OrderReportReply{Rows: make([]*ops.OrderReportRow, 0, len(result.Rows))}
	for _, row := range result.Rows {
		reply.Rows = append(reply.Rows, &ops.OrderReportRow{
			Date:     row.Date,
			UserId:   row.UserID,
			Nickname: row.Nickname,
			CostFen:  row.CostFen,
		})
	}
	return reply, nil
}
