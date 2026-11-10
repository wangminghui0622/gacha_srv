package logic

import (
	"context"
	"errors"

	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPaymentLogic {
	return &GetPaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetPayment 支付单详情：订单头 + 资金腿列表（元）
func (l *GetPaymentLogic) GetPayment(in *money.GetPaymentReq) (*money.GetPaymentResp, error) {
	if in.RequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "requestId不能为空")
	}
	paymentNo := "P" + in.RequestId

	o, err := l.svcCtx.PaymentOrderModel.FindOneByPaymentNo(l.ctx, paymentNo)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodePaymentOrderNotFound, "支付单不存在")
		}
		l.Errorf("GetPayment find order failed, paymentNo: %s, err: %v", paymentNo, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, err.Error())
	}

	var legs []gacha_money.PaymentLegs
	if err := l.svcCtx.DB.Where("payment_no = ?", paymentNo).Find(&legs).Error; err != nil {
		l.Errorf("GetPayment find legs failed, paymentNo: %s, err: %v", paymentNo, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, err.Error())
	}

	items := make([]*money.PaymentLegItem, 0, len(legs))
	for _, leg := range legs {
		items = append(items, &money.PaymentLegItem{
			LegNo:      leg.LegNo,
			WalletKind: int32(leg.WalletKind),
			Amount:     utils.FenToYuan(leg.Amount),
			Status:     int32(leg.Status),
		})
	}
	l.Infof("GetPayment success, paymentNo: %s, legs: %d", paymentNo, len(legs))
	return &money.GetPaymentResp{
		PaymentNo:     paymentNo,
		Status:        int32(o.Status),
		GrossAmount:   utils.FenToYuan(o.GrossAmount),
		CashAmount:    utils.FenToYuan(o.CashAmount),
		LoyaltyAmount: utils.FenToYuan(o.LoyaltyAmount),
		Legs:          items,
	}, nil
}
