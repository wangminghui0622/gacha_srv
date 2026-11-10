// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package reporting

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/reporting"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"
)

// QueryUserProfitCurve 用户盈亏曲线（reporting.metric:read；
// GET /analytics/users/:userId/profit-curve?grain=&from=&to=&tenantId=）。
// 时间参数 RFC3339、grain 枚举名——与 server_v2 kratos 绑定一致。
func QueryUserProfitCurveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := restx.PathUint64(r, "userId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		tenantId, err := restx.QueryUint64(r, "tenantId")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		grain, err := restx.QueryEnum(r, "grain", ops.TimeGrain_value)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		from, err := restx.QueryTimestamp(r, "from")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		to, err := restx.QueryTimestamp(r, "to")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		req := &ops.QueryUserProfitCurveRequest{
			UserId:   userId,
			TenantId: tenantId,
			Grain:    ops.TimeGrain(grain),
			From:     from,
			To:       to,
		}
		resp, err := reporting.NewQueryUserProfitCurveLogic(r.Context(), svcCtx).QueryUserProfitCurve(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
