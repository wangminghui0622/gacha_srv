// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package money

import (
	"net/http"

	"gacha_srv/app/admin-gateway/internal/logic/money"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	moneypb "gacha_srv/app/money/money"
)

// ListRechargeOrders 充值订单列表（payment.order:read）。
func ListRechargeOrdersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := restx.QueryUint32(r, "status")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		minAmount, err := restx.QueryUint64(r, "minAmountFen")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		maxAmount, err := restx.QueryUint64(r, "maxAmountFen")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		startAt, err := restx.QueryUint64(r, "startAtUnixMillis")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		endAt, err := restx.QueryUint64(r, "endAtUnixMillis")
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
		req := &moneypb.ListRechargeOrdersReq{
			Keyword:           restx.QueryString(r, "keyword"),
			Status:            int32(status),
			RechargeType:      restx.QueryString(r, "rechargeType"),
			Platform:          restx.QueryString(r, "platform"),
			PayKeyword:        restx.QueryString(r, "payKeyword"),
			MinAmountFen:      int64(minAmount),
			MaxAmountFen:      int64(maxAmount),
			TimeField:         restx.QueryString(r, "timeField"),
			StartAtUnixMillis: int64(startAt),
			EndAtUnixMillis:   int64(endAt),
			Page:              int32(page),
			PageSize:          int32(pageSize),
		}
		resp, err := money.NewListRechargeOrdersLogic(r.Context(), svcCtx).ListRechargeOrders(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// RefundRechargeOrders 批量退款充值订单（payment.order:write）。
func RefundRechargeOrdersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req moneypb.RefundRechargeOrdersReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := money.NewRefundRechargeOrdersLogic(r.Context(), svcCtx).RefundRechargeOrders(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// ListRefundOrders 退款订单列表（payment.order:read）。
func ListRefundOrdersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := restx.QueryUint32(r, "status")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		minAmount, err := restx.QueryUint64(r, "minAmountFen")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		maxAmount, err := restx.QueryUint64(r, "maxAmountFen")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		startAt, err := restx.QueryUint64(r, "startAtUnixMillis")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		endAt, err := restx.QueryUint64(r, "endAtUnixMillis")
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
		req := &moneypb.ListRefundOrdersReq{
			Keyword:           restx.QueryString(r, "keyword"),
			AuditRemark:       restx.QueryString(r, "auditRemark"),
			Status:            int32(status),
			MinAmountFen:      int64(minAmount),
			MaxAmountFen:      int64(maxAmount),
			TimeField:         restx.QueryString(r, "timeField"),
			StartAtUnixMillis: int64(startAt),
			EndAtUnixMillis:   int64(endAt),
			Page:              int32(page),
			PageSize:          int32(pageSize),
		}
		resp, err := money.NewListRefundOrdersLogic(r.Context(), svcCtx).ListRefundOrders(req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// OperateRefundOrder 退款订单操作（payment.order:write）。
func OperateRefundOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := restx.PathUint64(r, "id")
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		var req moneypb.OperateRefundOrderReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		req.Id = int64(id)
		if claims := restx.ClaimsFrom(r.Context()); claims != nil && claims.Name != "" {
			req.Auditor = claims.Name
		}
		resp, err := money.NewOperateRefundOrderLogic(r.Context(), svcCtx).OperateRefundOrder(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// GetRefundAutoConfig 读取自动退款配置（payment.order:read）。
func GetRefundAutoConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := money.NewGetRefundAutoConfigLogic(r.Context(), svcCtx).GetRefundAutoConfig(&moneypb.GetRefundAutoConfigReq{})
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}

// UpdateRefundAutoConfig 保存自动退款配置（payment.order:write）。
func UpdateRefundAutoConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req moneypb.UpdateRefundAutoConfigReq
		if err := restx.ParseProto(r, &req); err != nil {
			restx.Error(w, r, err)
			return
		}
		resp, err := money.NewUpdateRefundAutoConfigLogic(r.Context(), svcCtx).UpdateRefundAutoConfig(&req)
		if err != nil {
			restx.Error(w, r, err)
			return
		}
		restx.WriteProto(w, r, resp)
	}
}
