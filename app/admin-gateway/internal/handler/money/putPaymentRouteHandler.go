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

// PutPaymentRoute 配置支付路由（payment.route:write；body=channel/scene/policy/
// candidates/expectedVersion；operatorUserId 取会话主体，不接受客户端传入——
// 照抄 server_v2 ConsolePaymentRouteService 契约）。
func PutPaymentRouteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req moneypb.PutPaymentRouteRequest
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		if err := restx.ValidateProto(&req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := money.NewPutPaymentRouteLogic(r.Context(), svcCtx).PutPaymentRoute(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
