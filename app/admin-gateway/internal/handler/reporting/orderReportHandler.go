package reporting

import (
	"net/http"
	"time"

	"gacha_srv/app/admin-gateway/internal/logic/reporting"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"
)

// OrderReportHandler 订单报表：
// GET /reporting/orders?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD&userId=&nickname=&phone=
func OrderReportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		end := time.Now().In(reportCST)
		start := end.AddDate(0, 0, -31)
		if raw := restx.QueryString(r, "endDate"); raw != "" {
			parsed, err := time.ParseInLocation("2006-01-02", raw, reportCST)
			if err != nil {
				restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "endDate 格式应为 YYYY-MM-DD"))
				return
			}
			end = parsed
		}
		if raw := restx.QueryString(r, "startDate"); raw != "" {
			parsed, err := time.ParseInLocation("2006-01-02", raw, reportCST)
			if err != nil {
				restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "startDate 格式应为 YYYY-MM-DD"))
				return
			}
			start = parsed
		}
		if end.Before(start) {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "开始日期不能晚于结束日期"))
			return
		}
		if end.Sub(start) > 366*24*time.Hour {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "查询区间不能超过 366 天"))
			return
		}

		resp, err := reporting.NewOrderReportLogic(r.Context(), svcCtx).Query(reporting.OrderReportFilter{
			Start:    start,
			End:      end,
			UserID:   restx.QueryString(r, "userId"),
			Nickname: restx.QueryString(r, "nickname"),
			Phone:    restx.QueryString(r, "phone"),
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteJSON(w, r, resp)
	}
}
