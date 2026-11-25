// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"
	"strconv"
	"strings"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefundUserBeansLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefundUserBeansLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundUserBeansLogic {
	return &RefundUserBeansLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *RefundUserBeansLogic) RefundBeans(userID int64, amount, requestID string) error {
	if userID <= 0 {
		return errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	amount = strings.TrimSpace(amount)
	value, err := strconv.ParseFloat(amount, 64)
	if err != nil || value <= 0 {
		return errorx.NewCodeError(errorx.CodeParamError, "退还金额须大于 0")
	}
	bizType := "admin_refund"
	bizID := "admin_refund_" + requestID
	freeze, err := l.svcCtx.MoneyRpc.FreezeWallet(l.ctx, &money.FreezeWalletReq{
		UserId:    userID,
		Kind:      2, // BONUS / 吉豆
		Amount:    amount,
		BizType:   bizType,
		BizId:     bizID,
		RequestId: requestID,
	})
	if err != nil {
		return err
	}
	if _, err := l.svcCtx.MoneyRpc.PostHold(l.ctx, &money.PostHoldReq{
		HoldNo:  freeze.HoldNo,
		BizType: bizType,
		BizId:   bizID,
	}); err != nil {
		_, _ = l.svcCtx.MoneyRpc.VoidHold(l.ctx, &money.VoidHoldReq{
			HoldNo:  freeze.HoldNo,
			BizType: bizType,
			BizId:   bizID,
		})
		return err
	}
	return nil
}
