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

// ListMetricDefinitions 指标定义列表（reporting.metric:read；GET /reporting/metrics）。
func ListMetricDefinitionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ops.ListMetricDefinitionsRequest
		resp, err := reporting.NewListMetricDefinitionsLogic(r.Context(), svcCtx).ListMetricDefinitions(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
