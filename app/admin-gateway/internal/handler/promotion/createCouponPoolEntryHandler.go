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

// CreateCouponPoolEntry 创建券池记录（promotion.coupon_pool:create；
// body 直接是 CouponPoolEntry 对象，id 由服务端生成）。
func CreateCouponPoolEntryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		var entry marketing.CouponPoolEntry
		if err := restx.ParseProto(r, &entry); err != nil {
			restx.Error(w, r, err)
			return
		}
		entry.Id = 0
		req := &marketing.UpsertCouponPoolEntryRequest{
			RequestId: restx.RequestIdFrom(r.Context()),
			TenantId:  1,
			Entry:     &entry,
		}
		resp, err := promotion.NewUpsertCouponPoolEntryLogic(r.Context(), svcCtx).UpsertCouponPoolEntry(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
