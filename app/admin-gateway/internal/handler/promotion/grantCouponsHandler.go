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

// GrantCoupons 发放优惠券（promotion.user_coupon:grant；requestId 取 X-Request-Id，
// source 固定 CONSOLE——照抄 server_v2）。
func GrantCouponsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		var req marketing.GrantCouponsRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.TenantId = 1
		req.RequestId = restx.RequestIdFrom(r.Context())
		req.Source = marketing.CouponGrantSource_COUPON_GRANT_SOURCE_CONSOLE
		resp, err := promotion.NewGrantCouponsLogic(r.Context(), svcCtx).GrantCoupons(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
