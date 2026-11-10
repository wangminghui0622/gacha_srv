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

// CreatePolicy 创建条件政策草稿（eligibility.policy:create；tenant 固定 1）。
func CreatePolicyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req base.CreatePolicyDraftRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.TenantId = 1
		resp, err := eligibility.NewCreatePolicyLogic(r.Context(), svcCtx).CreatePolicy(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
