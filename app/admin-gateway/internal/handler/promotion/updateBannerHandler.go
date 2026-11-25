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

func UpdateBannerHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := restx.PathUint64(r, "id")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var banner marketing.Banner
		if err := restx.ParseProto(r, &banner); err != nil {
			restx.Error(w, r, err)
			return
		}
		banner.Id = id
		resp, err := promotion.NewUpsertBannerLogic(r.Context(), svcCtx).UpsertBanner(&marketing.UpsertBannerRequest{
			RequestId: restx.RequestIdFrom(r.Context()),
			TenantId:  1,
			Banner:    &banner,
		})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
