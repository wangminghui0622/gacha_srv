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

// GetPolicy 条件政策详情（eligibility.policy:read；version 0=当前版本）。
func GetPolicyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		req := &base.GetPolicyRequest{TenantId: 1, PolicyId: policyId, Version: version}
		resp, err := eligibility.NewGetPolicyLogic(r.Context(), svcCtx).GetPolicy(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
