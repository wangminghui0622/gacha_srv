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

// PublishDashboard 发布看板（reporting.dashboard:publish；
// POST /reporting/dashboards/:dashboardId/publish；body: {expectedVersion}）。
func PublishDashboardHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dashboardId, err := restx.PathUint64(r, "dashboardId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req ops.PublishDashboardRequest
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.DashboardId = dashboardId
		resp, err := reporting.NewPublishDashboardLogic(r.Context(), svcCtx).PublishDashboard(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
