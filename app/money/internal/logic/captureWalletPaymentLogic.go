package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CaptureWalletPaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCaptureWalletPaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CaptureWalletPaymentLogic {
	return &CaptureWalletPaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CaptureWalletPayment SAGA 正向分支 07（末步）：机械状态迁移（PostHold×2 + CommitQuote 核销报价 + 订单 PAID）。
// 业务失败前置（04 冻结成功即足额），此处 Aborted 属防御语义（状态不满足 2006）。
// PostHold/CommitQuote 均幂等：DTM 重试/并发重复直接收敛。
//
// 错误语义：业务失败 → codes.Aborted（逆序补偿 05/04）；系统错误 → codes.Internal（DTM 重试）。
func (l *CaptureWalletPaymentLogic) CaptureWalletPayment(in *money.CaptureWalletPaymentReq) (*money.CaptureWalletPaymentResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=requestId不能为空")
	}
	paymentNo := "P" + in.RequestId

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("CaptureWalletPayment get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("CaptureWalletPayment get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var statusOut int32
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)
		// FOR UPDATE 锁单 + 状态机校验
		var order gacha_money.PaymentOrders
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("payment_no = ?", paymentNo).First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.Aborted, "code=2005 msg=支付单不存在")
		}
		if err != nil {
			return err
		}
		switch order.Status {
		case orderStatusPaid:
			statusOut = int32(orderStatusPaid)
			l.Infof("CaptureWalletPayment idempotent: order already paid, paymentNo: %s", paymentNo)
			return nil
		case orderStatusCreated, orderStatusCanceled, orderStatusRefunding, orderStatusRefunded:
			return status.Error(codes.Aborted, "code=2006 msg=支付单状态不满足捕获")
		}

		// 事务外 sub-call：逐腿 PostHold（幂等）；随后腿 PENDING→POSTED
		var legs []gacha_money.PaymentLegs
		if err := db.Where("payment_no = ?", paymentNo).Find(&legs).Error; err != nil {
			return err
		}
		for _, leg := range legs {
			if leg.Status == legStatusPosted {
				continue
			}
			if leg.Status != legStatusPending {
				return status.Error(codes.Aborted, "code=2006 msg=资金腿状态异常")
			}
			_, err := NewPostHoldLogic(l.ctx, l.svcCtx).PostHold(&money.PostHoldReq{
				HoldNo:  leg.HoldNo,
				BizType: "payment_leg",
				BizId:   leg.LegNo,
			})
			if err != nil {
				var ce *errorx.CodeError
				if errors.As(err, &ce) {
					return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
				}
				return err
			}
			if err := db.Model(&gacha_money.PaymentLegs{}).
				Where("leg_no = ? AND status = ?", leg.LegNo, legStatusPending).
				Update("status", legStatusPosted).Error; err != nil {
				return err
			}
		}
		// 核销报价（M10 起）：sub-call marketing.CommitQuote（幂等，USED 直接成功）。
		// 失败语义：业务 Aborted（报价状态不满足）→ 本分支 Aborted 走补偿（03 ReleaseQuote 释放券）；
		// 系统错误 → Internal 触发 DTM 重试（CommitQuote 幂等收敛）。
		if in.QuoteToken != "" {
			if _, err := l.svcCtx.MarketingRpc.CommitQuote(l.ctx, &marketingClient.CommitQuoteReq{
				QuoteToken: in.QuoteToken,
			}); err != nil {
				var ce *errorx.CodeError
				if errors.As(err, &ce) {
					return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
				}
				l.Errorf("CaptureWalletPayment CommitQuote failed, quoteToken: %s, err: %v", in.QuoteToken, err)
				return err
			}
		}
		// 订单 AWAITING→PAID（行已锁，盲更新防御）
		result := db.Model(&gacha_money.PaymentOrders{}).
			Where("payment_no = ? AND status = ?", paymentNo, orderStatusAwaiting).
			Update("status", orderStatusPaid)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return status.Error(codes.Aborted, "code=2006 msg=支付单状态不满足捕获")
		}
		statusOut = int32(orderStatusPaid)
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("CaptureWalletPayment aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("CaptureWalletPayment failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读库如实返回
		return l.readBackOrder(paymentNo)
	}
	l.Infof("CaptureWalletPayment success, requestId: %s, paymentNo: %s", in.RequestId, paymentNo)
	return &money.CaptureWalletPaymentResp{PaymentNo: paymentNo, Status: statusOut}, nil
}

// readBackOrder 屏障跳过时读库如实返回当前状态
func (l *CaptureWalletPaymentLogic) readBackOrder(paymentNo string) (*money.CaptureWalletPaymentResp, error) {
	o, err := l.svcCtx.PaymentOrderModel.FindOneByPaymentNo(l.ctx, paymentNo)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			l.Infof("readBackOrder: order not found, paymentNo: %s", paymentNo)
			return &money.CaptureWalletPaymentResp{PaymentNo: paymentNo}, nil
		}
		l.Errorf("readBackOrder failed, paymentNo: %s, err: %v", paymentNo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackOrder: order status %d, paymentNo: %s", o.Status, paymentNo)
	return &money.CaptureWalletPaymentResp{PaymentNo: paymentNo, Status: int32(o.Status)}, nil
}
