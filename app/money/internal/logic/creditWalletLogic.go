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
	"gorm.io/gorm"
)

type CreditWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreditWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreditWalletLogic {
	return &CreditWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreditWallet 钱包入账（本地事务，非 SAGA 分支，M7）：
// 幂等由流水号唯一键保证（"c_"+requestId），重复提交直接返回当前余额
func (l *CreditWalletLogic) CreditWallet(in *money.CreditWalletReq) (*money.CreditWalletResp, error) {
	if in.UserId <= 0 || in.RequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID和requestId不能为空")
	}
	if kind := int64(in.Kind); kind < walletKindCash || kind > walletKindLoyaltyPoint {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "钱包类型无效（1-4）")
	}
	fen, err := utils.YuanToFen(in.Amount)
	if err != nil || fen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "入账金额无效（正数、最多两位小数）")
	}
	kind := int64(in.Kind)
	logNo := "c_" + in.RequestId

	// 幂等快路径：同 requestId 已入账，直接返回当前余额
	if _, err := l.svcCtx.WalletLedgerModel.FindOneByLogNo(l.ctx, logNo); err == nil {
		return l.currentKindBalance(in.UserId, kind)
	}

	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		return creditWalletTx(l.ctx, tx, l.svcCtx.WalletAccountModel, l.svcCtx.WalletLedgerModel,
			in.UserId, kind, fen, logNo, "credit", in.RequestId)
	})
	if err != nil {
		if errors.Is(err, errWalletDuplicated) {
			// 并发重复请求：先到者已入账，本请求直接返回当前余额
			return l.currentKindBalance(in.UserId, kind)
		}
		l.Errorf("CreditWallet failed, userId: %d, kind: %d, err: %v", in.UserId, in.Kind, err)
		return nil, errorx.FromMySQLError(err)
	}
	l.Infof("CreditWallet success, userId: %d, kind: %d, amount: %d, requestId: %s", in.UserId, in.Kind, fen, in.RequestId)
	return l.currentKindBalance(in.UserId, kind)
}

// currentKindBalance 查询指定钱包可用余额（账户不存在按 0 返回）
func (l *CreditWalletLogic) currentKindBalance(userId, kind int64) (*money.CreditWalletResp, error) {
	wa, err := l.svcCtx.WalletAccountModel.FindOneByUserIdWalletKind(l.ctx, userId, kind)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			return &money.CreditWalletResp{Balance: utils.FenToYuan(0)}, nil
		}
		l.Errorf("CreditWallet query wallet failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromMySQLError(err)
	}
	return &money.CreditWalletResp{Balance: utils.FenToYuan(wa.Available)}, nil
}
