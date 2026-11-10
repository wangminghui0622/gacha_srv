// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package risk

import (
	"net/http"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/logic/risk"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"
)

// ClaimRiskCase 认领风控案件（risk.case:claim；无 body，operationId 取
// Idempotency-Key 头——照抄 server_v2；认领人 = 会话主体）。
func ClaimRiskCaseHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caseId := restx.PathValue(r, "caseId")
		operationId := r.Header.Get("Idempotency-Key")
		if operationId == "" {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeConsoleIdempotencyRequired, "缺少 Idempotency-Key 头"))
			return
		}
		req := &base.ClaimRiskCaseRequest{CaseId: caseId, OperationId: operationId}
		resp, err := risk.NewClaimRiskCaseLogic(r.Context(), svcCtx).ClaimRiskCase(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
