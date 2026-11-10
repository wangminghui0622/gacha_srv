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

type GetBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBalanceLogic {
	return &GetBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetBalance 查询 CASH 钱包可用余额（元字符串，两位小数；M7 起读钱包）。
// 用户存在性检查属 identity 域，拆分后不再重复校验（service-division.md §3.6）：
// 未开户按 0 返回（惰性开户见 ensureWalletAccount）。
func (l *GetBalanceLogic) GetBalance(in *money.GetBalanceReq) (*money.GetBalanceResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	wa, err := l.svcCtx.WalletAccountModel.FindOneByUserIdWalletKind(l.ctx, in.UserId, walletKindCash)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			// 无 CASH 账户行：按 0 返回
			return &money.GetBalanceResp{Balance: utils.FenToYuan(0)}, nil
		}
		l.Errorf("GetBalance failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.FromMySQLError(err)
	}
	return &money.GetBalanceResp{Balance: utils.FenToYuan(wa.Available)}, nil
}
