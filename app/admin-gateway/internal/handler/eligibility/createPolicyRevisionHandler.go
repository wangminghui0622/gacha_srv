// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package eligibility

import (
	"net/http"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/logic/eligibility"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// CreatePolicyRevision 创建条件政策修订（eligibility.policy:write；expectedLatestVersion CAS）。
func CreatePolicyRevisionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		policyId, err := restx.PathUint64(r, "policyId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req base.CreatePolicyRevisionRequest
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.PolicyId = policyId
		req.TenantId = 1
		resp, err := eligibility.NewCreatePolicyRevisionLogic(r.Context(), svcCtx).CreatePolicyRevision(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
