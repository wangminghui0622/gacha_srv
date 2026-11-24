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

// GetCouponPoolEntry 查看券池记录（promotion.coupon_pool:read）。
func GetCouponPoolEntryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		id, err := restx.PathUint64(r, "entryId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &marketing.GetCouponPoolEntryRequest{TenantId: 1, Id: id}
		resp, err := promotion.NewGetCouponPoolEntryLogic(r.Context(), svcCtx).GetCouponPoolEntry(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
