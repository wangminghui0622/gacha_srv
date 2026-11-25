// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"encoding/json"
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/customer"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"
)

func RefundUserBeansHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := restx.PathUint64(r, "id")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req struct {
			Amount string `json:"amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			restx.Error(w, r, errorx.NewCodeError(errorx.CodeParamError, "请求体无效"))
			return
		}
		if err := customer.NewRefundUserBeansLogic(r.Context(), svcCtx).RefundBeans(
			int64(id), req.Amount, restx.RequestIdFrom(r.Context()),
		); err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteJSON(w, r, map[string]any{"ok": true})
	}
}
