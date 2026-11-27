package reporting

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/reporting"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// RealtimeReportHandler 实时报表：
// GET /reporting/realtime
func RealtimeReportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := reporting.NewRealtimeReportLogic(r.Context(), svcCtx).Query()
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteJSON(w, r, resp)
	}
}
