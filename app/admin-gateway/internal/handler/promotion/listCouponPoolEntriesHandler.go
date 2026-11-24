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

// ListCouponPoolEntries 券池列表（promotion.coupon_pool:read；分页）。
func ListCouponPoolEntriesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
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
		req := &marketing.ListCouponPoolEntriesRequest{
			TenantId: 1,
			Page:     page,
			PageSize: pageSize,
		}
		resp, err := promotion.NewListCouponPoolEntriesLogic(r.Context(), svcCtx).ListCouponPoolEntries(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
