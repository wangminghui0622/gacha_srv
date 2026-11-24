// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/promotion"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"
)

// BatchGrantCoupons 按累计消费区间批量发券（promotion.user_coupon:grant；
// X-Request-Id 作为批次号，每人发券幂等）。
func BatchGrantCouponsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		var req marketing.BatchGrantCouponsRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.TenantId = 1
		req.RequestId = restx.RequestIdFrom(r.Context())
		resp, err := promotion.NewBatchGrantCouponsLogic(r.Context(), svcCtx).BatchGrantCoupons(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
