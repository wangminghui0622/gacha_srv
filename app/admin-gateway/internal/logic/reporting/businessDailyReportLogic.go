package reporting

import (
	"context"
	"time"

	"gacha_srv/app/admin-gateway/internal/report"
	"gacha_srv/app/admin-gateway/internal/svc"
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

// Query 经营日报：累计/本月汇总 + 指定日期区间每日明细。
func (l *BusinessDailyReportLogic) Query(start, end time.Time) (*report.Result, error) {
	if l.svcCtx.DB == nil {
		return nil, errorx.NewCodeError(errorx.CodeServerError, "报表数据库未配置")
	}
	result, err := report.QueryDaily(l.svcCtx.DB, start, end)
	if err != nil {
		l.Errorf("BusinessDailyReport failed, start: %s, end: %s, err: %v", start.Format("2006-01-02"), end.Format("2006-01-02"), err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "经营日报查询失败")
	}
	return result, nil
}
