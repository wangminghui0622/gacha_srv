// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package risk

import (
	"net/http"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/logic/risk"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// ListRiskCases 风控案件列表（risk.case:list；游标分页；actorId 可选过滤，0=全部）。
func ListRiskCasesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorId, err := restx.QueryUint64(r, "actorId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		status, err := restx.QueryUint32(r, "status")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		beforeMillis, err := restx.QueryUint64(r, "beforeCreatedAtUnixMillis")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &base.ListRiskCasesRequest{
			TenantId:                  1,
			ActorId:                   actorId,
			Status:                    base.RiskCaseStatus(status),
			PageSize:                  pageSize,
			BeforeCreatedAtUnixMillis: int64(beforeMillis),
			BeforeCaseId:              r.URL.Query().Get("beforeCaseId"),
		}
		resp, err := risk.NewListRiskCasesLogic(r.Context(), svcCtx).ListRiskCases(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
