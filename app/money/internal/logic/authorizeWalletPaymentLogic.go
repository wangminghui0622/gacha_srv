package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AuthorizeWalletPaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthorizeWalletPaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthorizeWalletPaymentLogic {
	return &AuthorizeWalletPaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AuthorizeWalletPayment SAGA 正向分支 04：建单+两腿，事务外本服务 FreezeWallet 直调冻结
// CASH/LOYALTY 两腿（内嵌非独立分支；旧仓为跨服务 user-rpc，合并后同服务本地调用）。
//
// 幂等：payment_no="P"+requestId、leg_no=payment_no+"_"+wallet_kind 确定性派生；
// DTM 重试/并发重复 → uk 撞键读回已有单/腿 → FreezeWallet 自身 uk(biz_type,biz_id) 幂等 → 收敛。
//
// 错误语义（SAGA 分支铁律）：业务失败（余额不足等）→ codes.Aborted 触发补偿；
// 系统错误 → codes.Internal，DTM 退避重试。
func (l *AuthorizeWalletPaymentLogic) AuthorizeWalletPayment(in *money.AuthorizeWalletPaymentReq) (*money.AuthorizeWalletPaymentResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.BoxId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=requestId/userId/boxId不能为空")
	}
	grossFen, err := utils.YuanToFen(in.GrossAmount)
	// gross=0 合法：券全抵/免单券的实付为 0（M10 免单订单：无腿、无冻结，捕获直接 PAID）
	if err != nil || grossFen < 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=订单总额无效")
	}
	cashFen, err := utils.YuanToFen(in.CashAmount)
	if err != nil || cashFen < 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=现金金额无效")
	}
	loyaltyFen, err := utils.YuanToFen(in.LoyaltyAmount)
	if err != nil || loyaltyFen < 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=积分抵现金额无效")
	}
	if grossFen != cashFen+loyaltyFen {
		return nil, status.Error(codes.Aborted, "code=400 msg=订单总额与分项金额不匹配")
	}
	paymentNo := "P" + in.RequestId

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("AuthorizeWalletPayment get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("AuthorizeWalletPayment get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var statusOut int32
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 建单：FOR UPDATE 锁单；uk_payment_no 撞键=重发/并发，读回已有单
		var order gacha_money.PaymentOrders
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("payment_no = ?", paymentNo).First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			now := time.Now()
			order = gacha_money.PaymentOrders{
				PaymentNo:          paymentNo,
				OrderNo:            in.RequestId,
				UserId:             in.UserId,
				BoxId:              in.BoxId,
				Status:             orderStatusCreated,
				GrossAmount:        grossFen,
				CashAmount:         cashFen,
				LoyaltyAmount:      loyaltyFen,
				LoyaltyPointsDebit: in.LoyaltyPointsDebit,
				Currency:           "CNY",
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			if err := l.svcCtx.PaymentOrderModel.Insert(l.ctx, db, &order); err != nil {
				if !errors.Is(err, gorm.ErrDuplicatedKey) {
					return err
				}
				// 并发同单：先到者已提交，读回（锁等待对方事务释放后可见）
				if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("payment_no = ?", paymentNo).First(&order).Error; err != nil {
					return err
				}
			}
		} else if err != nil {
			return err
		}
		if order.Status != orderStatusCreated {
			// 已成功执行过（AWAITING）：幂等返回当前状态（屏障正常不会走到这，防御）
			statusOut = int32(order.Status)
			return nil
		}

		// ② 建腿：确定性 leg_no，uk 幂等
		type legSpec struct {
			legNo  string
			kind   int64
			amount int64
		}
		specs := make([]legSpec, 0, 2)
		if cashFen > 0 {
			specs = append(specs, legSpec{paymentNo + "_1", walletKindCash, cashFen})
		}
		if loyaltyFen > 0 {
			specs = append(specs, legSpec{paymentNo + "_4", walletKindLoyaltyPoint, loyaltyFen})
		}
		legs := make([]gacha_money.PaymentLegs, 0, len(specs))
		for _, sp := range specs {
			var leg gacha_money.PaymentLegs
			err := db.Where("leg_no = ?", sp.legNo).First(&leg).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				now := time.Now()
				leg = gacha_money.PaymentLegs{
					LegNo:      sp.legNo,
					PaymentNo:  paymentNo,
					WalletKind: sp.kind,
					Amount:     sp.amount,
					Status:     legStatusPending,
					CreatedAt:  now,
					UpdatedAt:  now,
				}
				if err := l.svcCtx.PaymentLegModel.Insert(l.ctx, db, &leg); err != nil {
					if !errors.Is(err, gorm.ErrDuplicatedKey) {
						return err
					}
					if err := db.Where("leg_no = ?", sp.legNo).First(&leg).Error; err != nil {
						return err
					}
				}
			} else if err != nil {
				return err
			}
			legs = append(legs, leg)
		}

		// ③ 冻结（sub-call，自身 uk 幂等）：对 PENDING 且未冻结的腿逐条 FreezeWallet
		for i := range legs {
			leg := &legs[i]
			if leg.Status != legStatusPending || leg.HoldNo != "" {
				continue
			}
			fr, err := NewFreezeWalletLogic(l.ctx, l.svcCtx).FreezeWallet(&money.FreezeWalletReq{
				UserId:    in.UserId,
				Kind:      int32(leg.WalletKind),
				Amount:    utils.FenToYuan(leg.Amount),
				BizType:   "payment_leg",
				BizId:     leg.LegNo,
				RequestId: in.RequestId,
			})
			if err != nil {
				// 下游业务码还原为 Aborted（触发补偿）；其余系统错误 → DTM 重试
				var ce *errorx.CodeError
				if errors.As(err, &ce) {
					return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
				}
				return err
			}
			if err := db.Model(&gacha_money.PaymentLegs{}).
				Where("leg_no = ?", leg.LegNo).
				Update("hold_no", fr.HoldNo).Error; err != nil {
				return err
			}
		}

		// ④ CREATED→AWAITING_PAYMENT（行已锁，盲更新防御）
		result := db.Model(&gacha_money.PaymentOrders{}).
			Where("payment_no = ? AND status = ?", paymentNo, orderStatusCreated).
			Update("status", orderStatusAwaiting)
		if result.Error != nil {
			return result.Error
		}
		statusOut = int32(orderStatusAwaiting)
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("AuthorizeWalletPayment aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("AuthorizeWalletPayment failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（branch 已成功/重试/重复补偿）：读库如实返回
		return l.readBackOrder(paymentNo)
	}
	l.Infof("AuthorizeWalletPayment success, requestId: %s, paymentNo: %s", in.RequestId, paymentNo)
	return &money.AuthorizeWalletPaymentResp{PaymentNo: paymentNo, Status: statusOut}, nil
}

// readBackOrder 屏障跳过时读库如实返回当前状态
func (l *AuthorizeWalletPaymentLogic) readBackOrder(paymentNo string) (*money.AuthorizeWalletPaymentResp, error) {
	o, err := l.svcCtx.PaymentOrderModel.FindOneByPaymentNo(l.ctx, paymentNo)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			l.Infof("readBackOrder: order not found, paymentNo: %s, treat as skipped", paymentNo)
			return &money.AuthorizeWalletPaymentResp{PaymentNo: paymentNo, Status: int32(orderStatusCreated)}, nil
		}
		l.Errorf("readBackOrder failed, paymentNo: %s, err: %v", paymentNo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackOrder: order status %d, paymentNo: %s", o.Status, paymentNo)
	return &money.AuthorizeWalletPaymentResp{PaymentNo: paymentNo, Status: int32(o.Status)}, nil
}
