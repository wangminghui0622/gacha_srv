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

// ListCouponTemplates 优惠券模板列表（promotion.coupon_template:read；游标分页）。
func ListCouponTemplatesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pathTenantID(r); err != nil {
			restx.Error(w, r, err)
			return
		}
		afterTemplateId, err := restx.QueryUint64(r, "afterTemplateId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		afterVersion, err := restx.QueryUint64(r, "afterVersion")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		pageSize, err := restx.QueryUint32(r, "pageSize")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &marketing.ListCouponTemplateVersionsRequest{
			TenantId:        1,
			AfterTemplateId: afterTemplateId,
			AfterVersion:    afterVersion,
			PageSize:        pageSize,
		}
		resp, err := promotion.NewListCouponTemplatesLogic(r.Context(), svcCtx).ListCouponTemplates(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
