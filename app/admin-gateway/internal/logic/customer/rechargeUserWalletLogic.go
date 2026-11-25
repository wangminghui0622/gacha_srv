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

type RechargeUserWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRechargeUserWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RechargeUserWalletLogic {
	return &RechargeUserWalletLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *RechargeUserWalletLogic) Recharge(userID int64, kind int32, amount, requestID string) (string, error) {
	if userID <= 0 {
		return "", errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	if kind < 1 || kind > 4 {
		return "", errorx.NewCodeError(errorx.CodeParamError, "该钱包暂不支持充值")
	}
	amount = strings.TrimSpace(amount)
	value, err := strconv.ParseFloat(amount, 64)
	if err != nil || value <= 0 {
		return "", errorx.NewCodeError(errorx.CodeParamError, "充值金额须大于 0")
	}
	resp, err := l.svcCtx.MoneyRpc.CreditWallet(l.ctx, &money.CreditWalletReq{
		UserId:    userID,
		Kind:      kind,
		Amount:    amount,
		RequestId: "admin_recharge_" + requestID,
	})
	if err != nil {
		return "", err
	}
	return resp.Balance, nil
}
