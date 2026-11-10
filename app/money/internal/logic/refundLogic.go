package logic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// errRefundDuplicated 退款幂等键撞键：同 refundRequestId 重复退款，读回已有退款单如实返回
var errRefundDuplicated = errors.New("refund already exists")

type RefundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundLogic {
	return &RefundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Refund 非 SAGA 的普通 RPC（运营/后台触发）：PAID 订单全额退款。
// 幂等：refund_no="R"+refundRequestId 确定性派生，uk 撞键=重复退款读回。
// 流程：锁单校验 PAID → 建退款单 → 逐腿退回钱包（CreditWallet 自身 uk 幂等）→ 腿 VOIDED → 单 REFUNDED → 退款单 SUCCEEDED。
func (l *RefundLogic) Refund(in *money.RefundReq) (*money.RefundResp, error) {
	if in.RequestId == "" || in.RefundRequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "requestId/refundRequestId不能为空")
	}
	paymentNo := "P" + in.RequestId
	refundNo := "R" + in.RefundRequestId

	// 幂等快路径：同退款键已成功过，直接返回
	if rf, err := l.svcCtx.PaymentRefundModel.FindOneByRefundNo(l.ctx, refundNo); err == nil {
		l.Infof("Refund idempotent hit, refundNo: %s, status: %d", refundNo, rf.Status)
		return &money.RefundResp{RefundNo: refundNo, Status: int32(rf.Status)}, nil
	} else if !errors.Is(err, gacha_money.ErrNotFound) {
		l.Errorf("Refund find refund failed, refundNo: %s, err: %v", refundNo, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, err.Error())
	}

	var statusOut int64
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		// 锁支付单 + 状态机：仅 PAID 可退
		var order gacha_money.PaymentOrders
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("payment_no = ?", paymentNo).First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewCodeError(errorx.CodePaymentOrderNotFound, "支付单不存在")
		}
		if err != nil {
			return err
		}
		if order.Status != orderStatusPaid {
			return errorx.NewCodeError(errorx.CodePaymentStateInvalid, "仅已支付订单可退款")
		}

		// 建退款单（uk_refund_no 撞键=并发重复退款，读回）
		now := time.Now()
		refund := &gacha_money.PaymentRefunds{
			RefundNo:  refundNo,
			PaymentNo: paymentNo,
			Amount:    order.GrossAmount,
			Status:    refundStatusProcessing,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := l.svcCtx.PaymentRefundModel.Insert(l.ctx, tx, refund); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errRefundDuplicated
			}
			return err
		}

		// 逐 POSTED 腿退回钱包（CreditWallet 幂等：log_no="c_"+requestId uk）
		var legs []gacha_money.PaymentLegs
		if err := tx.Where("payment_no = ?", paymentNo).Find(&legs).Error; err != nil {
			return err
		}
		for _, leg := range legs {
			if leg.Status != legStatusPosted {
				continue
			}
			_, err := NewCreditWalletLogic(l.ctx, l.svcCtx).CreditWallet(&money.CreditWalletReq{
				UserId:    order.UserId,
				Kind:      int32(leg.WalletKind),
				Amount:    utils.FenToYuan(leg.Amount),
				RequestId: refundNo + "_" + strconv.FormatInt(leg.WalletKind, 10),
			})
			if err != nil {
				return fmt.Errorf("credit wallet refund failed, legNo: %s, err: %w", leg.LegNo, err)
			}
			if err := tx.Model(&gacha_money.PaymentLegs{}).
				Where("leg_no = ? AND status = ?", leg.LegNo, legStatusPosted).
				Update("status", legStatusVoided).Error; err != nil {
				return err
			}
		}

		// 单 REFUNDED + 退款单 SUCCEEDED
		if err := tx.Model(&gacha_money.PaymentOrders{}).
			Where("payment_no = ? AND status = ?", paymentNo, orderStatusPaid).
			Update("status", orderStatusRefunded).Error; err != nil {
			return err
		}
		if err := tx.Model(&gacha_money.PaymentRefunds{}).
			Where("refund_no = ? AND status = ?", refundNo, refundStatusProcessing).
			Update("status", refundStatusSucceeded).Error; err != nil {
			return err
		}
		statusOut = refundStatusSucceeded
		return nil
	})
	if err != nil {
		if errors.Is(err, errRefundDuplicated) {
			// 并发重复退款：读回已有退款单
			return l.readBackRefund(refundNo)
		}
		if errorx.IsCodeError(err) {
			return nil, err
		}
		l.Errorf("Refund failed, refundNo: %s, err: %v", refundNo, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, err.Error())
	}
	l.Infof("Refund success, refundNo: %s, paymentNo: %s", refundNo, paymentNo)
	return &money.RefundResp{RefundNo: refundNo, Status: int32(statusOut)}, nil
}

// readBackRefund 幂等撞键后读库如实返回
func (l *RefundLogic) readBackRefund(refundNo string) (*money.RefundResp, error) {
	rf, err := l.svcCtx.PaymentRefundModel.FindOneByRefundNo(l.ctx, refundNo)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			l.Infof("readBackRefund: refund not found yet, refundNo: %s, treat as processing", refundNo)
			return &money.RefundResp{RefundNo: refundNo, Status: int32(refundStatusProcessing)}, nil
		}
		l.Errorf("readBackRefund failed, refundNo: %s, err: %v", refundNo, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, err.Error())
	}
	l.Infof("readBackRefund: refund status %d, refundNo: %s", rf.Status, refundNo)
	return &money.RefundResp{RefundNo: refundNo, Status: int32(rf.Status)}, nil
}
