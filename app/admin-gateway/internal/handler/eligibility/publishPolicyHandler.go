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

// PublishPolicy 发布条件政策（eligibility.policy:publish；body 取 expectedConfigHash，
// policyId/version 取路径，expectedConfigHash CAS）。
func PublishPolicyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		policyId, err := restx.PathUint64(r, "policyId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		version, err := restx.PathUint64(r, "version")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req base.PublishPolicyRequest
		if err = restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.PolicyId = policyId
		req.Version = version
		req.TenantId = 1
		resp, err := eligibility.NewPublishPolicyLogic(r.Context(), svcCtx).PublishPolicy(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
