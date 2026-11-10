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

// GetDashboard 读取看板（reporting.dashboard:read；GET /reporting/dashboards/:dashboardId）。
func GetDashboardHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dashboardId, err := restx.PathUint64(r, "dashboardId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &ops.GetDashboardRequest{DashboardId: dashboardId}
		resp, err := reporting.NewGetDashboardLogic(r.Context(), svcCtx).GetDashboard(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
