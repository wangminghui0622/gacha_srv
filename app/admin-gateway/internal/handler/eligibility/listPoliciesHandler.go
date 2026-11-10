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

// ListPolicies 条件政策列表（eligibility.policy:read；游标分页，tenant 固定 1）。
func ListPoliciesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		afterPolicyId, err := restx.QueryUint64(r, "afterPolicyId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &base.ListPoliciesRequest{TenantId: 1, AfterPolicyId: afterPolicyId, PageSize: pageSize}
		resp, err := eligibility.NewListPoliciesLogic(r.Context(), svcCtx).ListPolicies(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
