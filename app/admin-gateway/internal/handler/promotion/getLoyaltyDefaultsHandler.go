// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"net/http"

	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/admin-gateway/internal/logic/promotion"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// GetLoyaltyDefaults 查看忠诚度默认值（promotion.tenant_loyalty:read）。
func GetLoyaltyDefaultsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &marketing.GetTenantLoyaltyDefaultsRequest{TenantId: 1}
		resp, err := promotion.NewGetLoyaltyDefaultsLogic(r.Context(), svcCtx).GetLoyaltyDefaults(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
