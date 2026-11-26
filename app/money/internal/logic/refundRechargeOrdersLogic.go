package logic

import (
	"context"
	"errors"
	"strconv"
	"time"

	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type RefundRechargeOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefundRechargeOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundRechargeOrdersLogic {
	return &RefundRechargeOrdersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// RefundRechargeOrders 后台批量退款（记账态）：仅充值成功的订单可退。
// 说明：真实第三方退款在支付渠道接入后替换此处的状态迁移。
func (l *RefundRechargeOrdersLogic) RefundRechargeOrders(in *money.RefundRechargeOrdersReq) (*money.RefundRechargeOrdersResp, error) {
	if len(in.GetIds()) == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请选择要退款的充值订单")
	}
	resp := &money.RefundRechargeOrdersResp{}
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		for _, id := range in.GetIds() {
			row, err := repo.GetRechargeOrder(tx, id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					resp.FailedOrderNos = append(resp.FailedOrderNos, "#"+strconv.FormatInt(id, 10))
					continue
				}
				return err
			}
			if row.Status != int32(money.RechargeOrderStatus_RECHARGE_ORDER_STATUS_SUCCESS) &&
				row.Status != int32(money.RechargeOrderStatus_RECHARGE_ORDER_STATUS_REFUND_FAILED) {
				resp.FailedOrderNos = append(resp.FailedOrderNos, row.OrderNo)
				continue
			}
			now := time.Now()
			refundNo := "R" + row.OrderNo
			if err := repo.UpdateRechargeOrder(tx, row.Id, map[string]any{
				"status":            money.RechargeOrderStatus_RECHARGE_ORDER_STATUS_REFUNDED,
				"refunded_at":       &now,
				"refund_amount_fen": row.AmountFen,
				"refund_order_no":   refundNo,
			}); err != nil {
				return err
			}
			resp.Refunded++
		}
		return nil
	})
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return resp, nil
}
