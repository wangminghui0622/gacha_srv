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

type RechargeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRechargeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RechargeLogic {
	return &RechargeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Recharge mock 充值（本地事务，非 SAGA 分支，M7 起入 CASH 钱包）：
// 幂等由流水号唯一键保证（"c_"+requestId），重复提交直接返回当前余额
func (l *RechargeLogic) Recharge(in *money.RechargeReq) (*money.RechargeResp, error) {
	if in.UserId <= 0 || in.RequestId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID和requestId不能为空")
	}
	fen, err := utils.YuanToFen(in.Amount)
	if err != nil || fen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "充值金额无效（正数、最多两位小数）")
	}
	logNo := "c_" + in.RequestId

	// 幂等快路径：同 requestId 已入账，直接返回当前余额
	if _, err := l.svcCtx.WalletLedgerModel.FindOneByLogNo(l.ctx, logNo); err == nil {
		return l.currentBalance(in.UserId)
	}

	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		return creditWalletTx(l.ctx, tx, l.svcCtx.WalletAccountModel, l.svcCtx.WalletLedgerModel,
			in.UserId, walletKindCash, fen, logNo, "recharge", in.RequestId)
	})
	if err != nil {
		if errors.Is(err, errWalletDuplicated) {
			// 并发重复请求：先到者已入账，本请求直接返回当前余额
			return l.currentBalance(in.UserId)
		}
		l.Errorf("Recharge failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.FromMySQLError(err)
	}
	l.Infof("Recharge success, userId: %d, amount: %d, requestId: %s", in.UserId, fen, in.RequestId)
	return l.currentBalance(in.UserId)
}

// currentBalance 查询 CASH 钱包可用余额（账户不存在按 0 返回）
func (l *RechargeLogic) currentBalance(userId int64) (*money.RechargeResp, error) {
	wa, err := l.svcCtx.WalletAccountModel.FindOneByUserIdWalletKind(l.ctx, userId, walletKindCash)
	if err != nil {
		if errors.Is(err, gacha_money.ErrNotFound) {
			return &money.RechargeResp{Balance: utils.FenToYuan(0)}, nil
		}
		l.Errorf("Recharge query balance failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromMySQLError(err)
	}
	return &money.RechargeResp{Balance: utils.FenToYuan(wa.Available)}, nil
}
