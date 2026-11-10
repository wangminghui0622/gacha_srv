// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package reporting

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/reporting"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
)

// QueryDashboard 查询看板（reporting.dashboard:query；
// POST /reporting/dashboards/:dashboardId/query；body 为查询参数，
// tenant 由会话绑定 + 数据范围守卫（ORGANIZATION/ALL））。
func QueryDashboardHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dashboardId, err := restx.PathUint64(r, "dashboardId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req ops.QueryDashboardRequest
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.DashboardId = dashboardId
		resp, err := reporting.NewQueryDashboardLogic(r.Context(), svcCtx).QueryDashboard(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
