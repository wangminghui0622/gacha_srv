// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/app/money/money"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserFinanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserFinanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserFinanceLogic {
	return &GetUserFinanceLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserFinanceLogic) GetUserFinance(in *identity.GetUserFinanceReq) (*identity.UserFinance, error) {
	finance, err := l.svcCtx.IdentityRpc.GetUserFinance(l.ctx, in)
	if err != nil {
		return nil, err
	}
	wallet, err := l.svcCtx.MoneyRpc.GetWallet(l.ctx, &money.GetWalletReq{UserId: in.UserId})
	if err != nil {
		return nil, err
	}
	balances := make(map[int32]string, len(wallet.Kinds))
	for _, item := range wallet.Kinds {
		balances[item.Kind] = item.Available
	}
	for _, item := range finance.Wallets {
		if balance, ok := balances[item.Kind]; ok {
			item.Balance = balance
		} else {
			item.Balance = "0"
		}
	}
	return finance, nil
}
