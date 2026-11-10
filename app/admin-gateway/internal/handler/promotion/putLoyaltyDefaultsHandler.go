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

// PutLoyaltyDefaults 配置忠诚度默认值（promotion.tenant_loyalty:write；
// body 直接是 defaults 对象——照抄 kratos body:"defaults"）。
func PutLoyaltyDefaultsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		var defaults marketing.TenantLoyaltyDefaults
		if err := restx.ParseProto(r, &defaults); err != nil {
			restx.Error(w, r, err)
			return
		}
		defaults.TenantId = 1
		req := &marketing.PutTenantLoyaltyDefaultsRequest{Defaults: &defaults}
		resp, err := promotion.NewPutLoyaltyDefaultsLogic(r.Context(), svcCtx).PutLoyaltyDefaults(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
