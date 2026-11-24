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

// UpdateCouponPoolEntry 编辑券池记录（promotion.coupon_pool:write；
// body 直接是 CouponPoolEntry 对象，路径 entryId 优先）。
func UpdateCouponPoolEntryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
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
		var entry marketing.CouponPoolEntry
		if err := restx.ParseProto(r, &entry); err != nil {
			restx.Error(w, r, err)
			return
		}
		entry.Id = id
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
