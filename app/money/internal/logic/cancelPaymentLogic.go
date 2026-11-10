package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"
	"strconv"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/app/play/playClient"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CancelPaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelPaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelPaymentLogic {
	return &CancelPaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CancelPayment SAGA 分支 04 补偿：解冻全部 PENDING 腿、订单 CANCELED。
//
// 跳过情形（均如实区分）：
//  1. 屏障级跳过：读库如实返回；
//  2. 订单不存在 = 空补偿（正向建单前即失败）；
//  3. 订单已 CANCELED = 重复补偿。
func (l *CancelPaymentLogic) CancelPayment(in *money.CancelPaymentReq) (*money.CancelPaymentResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=requestId不能为空")
	}
	paymentNo := "P" + in.RequestId

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("CancelPayment get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("CancelPayment get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var statusOut int32
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)
		// FOR UPDATE 锁单（不过滤 status：如实区分「已取消=重复补偿」与「不存在=空补偿」）
		var order gacha_money.PaymentOrders
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payment_no = ?", paymentNo).First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 空补偿 + 泄漏清扫：正向 barrier 事务在冻结 sub-call 成功后整体回滚时，
			// user 侧 hold 可能已落库而 payment 侧无单/无腿。hold_no 确定性派生
			// （"h_"+legNo，legNo=paymentNo+"_"+walletKind），逐一按确定性键解冻兜底；
			// VoidHold 幂等：hold 不存在 → Aborted 2003 跳过，存在 → 解冻回账。
			statusOut = int32(orderStatusCanceled)
			if err := l.sweepOrphanHolds(paymentNo); err != nil {
				return err
			}
			l.Infof("empty compensate: order not found, paymentNo: %s (forward op never ran), holds swept, skip", paymentNo)
			return nil
		}
		if err != nil {
			return fmt.Errorf("select payment order failed: %w", err)
		}
		switch order.Status {
		case orderStatusCanceled:
			statusOut = int32(orderStatusCanceled)
			l.Infof("empty compensate: order already canceled, paymentNo: %s, skip", paymentNo)
			return nil
		case orderStatusPaid, orderStatusRefunding, orderStatusRefunded:
			// 防御：正向已走到捕获/退款，04 补偿不应发生；返回 Internal 让 DTM 重试暴露
			return fmt.Errorf("order in terminal state %d, cancel rejected, paymentNo: %s", order.Status, paymentNo)
		}

		// ── 高1 修复：发奖落库退款前置检查（发奖与退款对称）──
		// 该 gid 若已有发奖成功落库（play 域五链 prize_grant_marks / money 域 draw 资产），
		// 则不执行 VoidHold 退钱：奖品不追回、钱保持扣除（冻结保持可恢复，PostHold 幂等，
		// 后续对账/人工可补入账），直接返回成功让 SAGA 补偿正常完结——用户不能白拿奖品；
		// 反之（未发奖）退款照常，退款成功则奖不追。查询 RPC 故障返回 Internal → DTM
		// 重试补偿（重复补偿幂等），安全。
		if grantsResp, err := l.svcCtx.PlayRpc.CheckPrizesGranted(l.ctx, &playClient.CheckPrizesGrantedReq{RequestId: in.RequestId}); err != nil {
			return err
		} else if grantsResp.Granted {
			l.Errorf("CancelPayment skip refund: prizes already granted, keep money deducted (holds kept recoverable), requestId: %s, paymentNo: %s",
				in.RequestId, paymentNo)
			statusOut = int32(order.Status)
			return nil
		}
		if grantsResp, err := NewCheckGrantsByRequestIdLogic(l.ctx, l.svcCtx).CheckGrantsByRequestId(&money.CheckGrantsByRequestIdReq{RequestId: in.RequestId}); err != nil {
			return err
		} else if grantsResp.Granted {
			l.Errorf("CancelPayment skip refund: draw assets already granted, keep money deducted (holds kept recoverable), requestId: %s, paymentNo: %s", in.RequestId, paymentNo)
			statusOut = int32(order.Status)
			return nil
		}

		// 解冻全部 PENDING 腿（VoidHold 幂等：VOIDED→成功）
		var legs []gacha_money.PaymentLegs
		if err := db.Where("payment_no = ?", paymentNo).Find(&legs).Error; err != nil {
			return err
		}
		for _, leg := range legs {
			if leg.Status != legStatusPending {
				continue
			}
			if leg.HoldNo == "" {
				// 冻结 sub-call 尚未执行（正向在冻结前失败）：腿直接 VOIDED，无冻结可解
				if err := db.Model(&gacha_money.PaymentLegs{}).Where("leg_no = ? AND status = ?", leg.LegNo, legStatusPending).Update("status", legStatusVoided).Error; err != nil {
					return err
				}
				continue
			}
			_, err := NewVoidHoldLogic(l.ctx, l.svcCtx).VoidHold(&money.VoidHoldReq{
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
			if err := db.Model(&gacha_money.PaymentLegs{}).Where("leg_no = ? AND status = ?", leg.LegNo, legStatusPending).Update("status", legStatusVoided).Error; err != nil {
				return err
			}
		}
		// 订单→CANCELED
		result := db.Model(&gacha_money.PaymentOrders{}).Where("payment_no = ?", paymentNo).Update("status", orderStatusCanceled)
		if result.Error != nil {
			return result.Error
		}
		statusOut = int32(orderStatusCanceled)
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("CancelPayment aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("CancelPayment failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读库如实返回
		return l.readBackOrder(paymentNo)
	}
	l.Infof("CancelPayment success, requestId: %s, status: %d", in.RequestId, statusOut)
	return &money.CancelPaymentResp{Status: statusOut}, nil
}

// sweepOrphanHolds 空补偿兜底：按确定性腿号解冻可能泄漏的 hold（正向事务回滚的窗口泄漏）。
// 逐钱包类型探测：VoidHold 对不存在的 hold 返回 Aborted 2003（跳过），其余 Aborted 透传。
func (l *CancelPaymentLogic) sweepOrphanHolds(paymentNo string) error {
	for _, kind := range []int64{walletKindCash, walletKindLoyaltyPoint} {
		legNo := paymentNo + "_" + strconv.FormatInt(kind, 10)
		_, err := NewVoidHoldLogic(l.ctx, l.svcCtx).VoidHold(&money.VoidHoldReq{
			HoldNo:  "h_" + legNo,
			BizType: "payment_leg",
			BizId:   legNo,
		})
		if err == nil {
			l.Infof("sweepOrphanHolds: voided orphan hold, legNo: %s", legNo)
			continue
		}
		var ce *errorx.CodeError
		if errors.As(err, &ce) {
			if ce.Code == errorx.CodeHoldNotFound {
				continue // 无泄漏，正常
			}
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
		}
		return err
	}
	return nil
}

// readBackOrder 屏障跳过时读库如实返回当前状态
func (l *CancelPaymentLogic) readBackOrder(paymentNo string) (*money.CancelPaymentResp, error) {
	o, err := l.svcCtx.PaymentOrderModel.FindOneByPaymentNo(l.ctx, paymentNo)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			l.Infof("readBackOrder: order not found, paymentNo: %s, treat as canceled", paymentNo)
			return &money.CancelPaymentResp{Status: int32(orderStatusCanceled)}, nil
		}
		l.Errorf("readBackOrder failed, paymentNo: %s, err: %v", paymentNo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackOrder: order status %d, paymentNo: %s", o.Status, paymentNo)
	return &money.CancelPaymentResp{Status: int32(o.Status)}, nil
}
