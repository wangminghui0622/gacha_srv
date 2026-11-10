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

// PublishCouponTemplate 发布券模板（promotion.coupon_template:publish；
// body 直接是 template 对象——照抄 kratos body:"template"；requestId 取
// X-Request-Id 头，同号同内容重放幂等）。
func PublishCouponTemplateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		var tpl marketing.CouponTemplate
		if err := restx.ParseProto(r, &tpl); err != nil {
			restx.Error(w, r, err)
			return
		}
		tpl.TenantId = 1
		req := &marketing.PublishCouponTemplateRequest{
			RequestId: restx.RequestIdFrom(r.Context()),
			Template:  &tpl,
		}
		resp, err := promotion.NewPublishCouponTemplateLogic(r.Context(), svcCtx).PublishCouponTemplate(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
