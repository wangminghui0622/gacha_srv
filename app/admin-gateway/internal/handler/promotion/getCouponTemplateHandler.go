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

// GetCouponTemplate 查看券模板（promotion.coupon_template:read）。
func GetCouponTemplateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		templateId, err := restx.PathUint64(r, "templateId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &marketing.GetCouponTemplateRequest{TenantId: 1, TemplateId: templateId}
		resp, err := promotion.NewGetCouponTemplateLogic(r.Context(), svcCtx).GetCouponTemplate(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
