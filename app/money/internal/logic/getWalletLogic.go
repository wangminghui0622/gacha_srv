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

type GetWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWalletLogic {
	return &GetWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetWallet 查询四类钱包余额（缺行按 0 返回，前端抵现开关/余额展示用）
func (l *GetWalletLogic) GetWallet(in *money.GetWalletReq) (*money.GetWalletResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	kindList := []int64{walletKindCash, walletKindBonus, walletKindRedeemPoint, walletKindLoyaltyPoint}
	kinds := make([]*money.WalletKindBalance, 0, len(kindList))
	for _, kind := range kindList {
		wa, err := l.svcCtx.WalletAccountModel.FindOneByUserIdWalletKind(l.ctx, in.UserId, kind)
		switch {
		case err == nil:
			kinds = append(kinds, &money.WalletKindBalance{
				Kind:      int32(kind),
				Available: utils.FenToYuan(wa.Available),
				Held:      utils.FenToYuan(wa.Held),
			})
		case errors.Is(err, gacha_money.ErrNotFound):
			// 缺行（未开过户）：按 0 返回
			kinds = append(kinds, &money.WalletKindBalance{
				Kind:      int32(kind),
				Available: utils.FenToYuan(0),
				Held:      utils.FenToYuan(0),
			})
		default:
			l.Errorf("GetWallet failed, userId: %d, kind: %d, err: %v", in.UserId, kind, err)
			return nil, errorx.FromMySQLError(err)
		}
	}
	return &money.GetWalletResp{Kinds: kinds}, nil
}
