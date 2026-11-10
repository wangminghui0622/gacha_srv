// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package money

import (
	"net/http"

	moneypb "gacha_srv/app/money/money"
	"gacha_srv/app/admin-gateway/internal/logic/money"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
)

// GetPaymentRoute 读取支付路由（money.route:read；query channel/scene 数值枚举）。
func GetPaymentRouteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channel, err := restx.QueryUint32(r, "channel")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		scene, err := restx.QueryUint32(r, "scene")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &moneypb.GetPaymentRouteRequest{
			Channel: moneypb.PaymentChannel(channel),
			Scene:   moneypb.PaymentScene(scene),
		}
		resp, err := money.NewGetPaymentRouteLogic(r.Context(), svcCtx).GetPaymentRoute(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
