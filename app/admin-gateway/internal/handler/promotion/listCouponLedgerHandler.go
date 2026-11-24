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

// ListCouponLedger 券池/用户券流水（promotion.coupon_pool:read；分页）。
func ListCouponLedgerHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		entryID, err := restx.QueryUint64(r, "entryId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		userCouponID, err := restx.QueryUint64(r, "userCouponId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		userID, err := restx.QueryUint64(r, "userId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		page, err := restx.QueryUint32(r, "page")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &marketing.ListCouponLedgerRequest{
			TenantId:     1,
			EntryId:      entryID,
			UserCouponId: userCouponID,
			UserId:       userID,
			OrderId:      restx.QueryString(r, "orderId"),
			Page:         page,
			PageSize:     pageSize,
		}
		resp, err := promotion.NewListCouponLedgerLogic(r.Context(), svcCtx).ListCouponLedger(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
