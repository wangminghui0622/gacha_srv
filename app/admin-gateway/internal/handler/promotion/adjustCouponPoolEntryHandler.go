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

// AdjustCouponPoolEntry 增减券池库存（promotion.coupon_pool:write；
// body={delta, remark}，路径 entryId 优先，操作人取当前登录管理员）。
func AdjustCouponPoolEntryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		entryID, err := restx.PathUint64(r, "entryId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req marketing.AdjustCouponPoolEntryRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.TenantId = 1
		req.EntryId = entryID
		req.OperatorType = "ADMIN"
		if claims := restx.ClaimsFrom(r.Context()); claims != nil {
			req.OperatorId = claims.ActorID
		}
		resp, err := promotion.NewAdjustCouponPoolEntryLogic(r.Context(), svcCtx).AdjustCouponPoolEntry(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
