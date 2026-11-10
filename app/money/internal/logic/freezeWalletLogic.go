package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type FreezeWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFreezeWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreezeWalletLogic {
	return &FreezeWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// FreezeWallet 钱包冻结（非 DTM 分支，但被 AuthorizeWalletPayment SAGA 分支 04 内嵌直调）：
// 返回 CodeError 业务码，调用方包装为 codes.Aborted（触发 04 补偿）；系统错误原样返回（DTM 重试）。
// 幂等由确定性键保证：hold_no="h_"+bizId、uk(biz_type,biz_id)，撞键=重发，返回已有 hold。
func (l *FreezeWalletLogic) FreezeWallet(in *money.FreezeWalletReq) (*money.FreezeWalletResp, error) {
	if in.UserId <= 0 || in.BizType == "" || in.BizId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID/bizType/bizId不能为空")
	}
	if kind := int64(in.Kind); kind < walletKindCash || kind > walletKindLoyaltyPoint {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "钱包类型无效")
	}
	fen, err := utils.YuanToFen(in.Amount)
	if err != nil || fen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "冻结金额无效")
	}
	kind := int64(in.Kind)
	holdNo := "h_" + in.BizId

	// 幂等快路径：同 biz 单已冻结，直接返回已有 hold
	if h, err := l.svcCtx.WalletHoldModel.FindOneByBizTypeBizId(l.ctx, in.BizType, in.BizId); err == nil {
		return &money.FreezeWalletResp{HoldNo: h.HoldNo, Frozen: utils.FenToYuan(h.Amount)}, nil
	}

	var expiresAt *time.Time
	if in.ExpiresAt > 0 {
		t := time.Unix(in.ExpiresAt, 0)
		expiresAt = &t
	}

	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		if err := ensureWalletAccount(l.ctx, tx, l.svcCtx.WalletAccountModel, in.UserId, kind); err != nil {
			return err
		}
		wa, err := lockWalletAccount(tx, in.UserId, kind)
		if err != nil {
			return err
		}
		if wa.Available < fen {
			return errorx.NewCodeError(errorx.CodeBalanceNotEnough, "余额不足")
		}
		// 盲更新：available-、held+（行已锁，RowsAffected 必为 1）
		if err := tx.Model(&gacha_money.WalletAccounts{}).Where("user_id = ? AND wallet_kind = ?", in.UserId, kind).UpdateColumns(map[string]interface{}{
			"available": gorm.Expr("available - ?", fen),
			"held":      gorm.Expr("held + ?", fen),
		}).Error; err != nil {
			return err
		}
		// 插冻结单（uk_hold_no、uk(biz_type,biz_id) 双保险，撞键=重发幂等）
		if err := l.svcCtx.WalletHoldModel.Insert(l.ctx, tx, &gacha_money.WalletHolds{
			HoldNo:     holdNo,
			UserId:     in.UserId,
			WalletKind: kind,
			Amount:     fen,
			Status:     holdStatusPending,
			BizType:    in.BizType,
			BizId:      in.BizId,
			ExpiresAt:  expiresAt,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errWalletDuplicated
			}
			return err
		}
		// 插冻结流水（log_no=holdNo 确定性，Amount 为负表示可用余额减少）
		if err := l.svcCtx.WalletLedgerModel.Insert(l.ctx, tx, &gacha_money.WalletLedgers{
			LogNo:      holdNo,
			UserId:     in.UserId,
			WalletKind: kind,
			Amount:     -fen,
			BizType:    "freeze",
			BizId:      in.BizId,
			HoldNo:     holdNo,
			Status:     "success",
			CreatedAt:  time.Now(),
		}); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errWalletDuplicated
			}
			return err
		}
		return nil
	})
	if err != nil {
		// 已含码的业务失败（余额不足）原样返回，调用方包装 Aborted 触发 SAGA 04 补偿
		if errorx.IsCodeError(err) {
			l.Errorf("FreezeWallet aborted, userId: %d, reason: %s", in.UserId, err.Error())
			return nil, err
		}
		if errors.Is(err, errWalletDuplicated) {
			// 撞键：先到者已冻结并提交，读回返回
			return l.readBackHold(holdNo)
		}
		l.Errorf("FreezeWallet failed, userId: %d, err: %v", in.UserId, err)
		return nil, err
	}
	l.Infof("FreezeWallet success, userId: %d, holdNo: %s, amount: %d", in.UserId, holdNo, fen)
	return &money.FreezeWalletResp{HoldNo: holdNo, Frozen: utils.FenToYuan(fen)}, nil
}

// readBackHold 撞键/重发时读回已有 hold 如实返回
func (l *FreezeWalletLogic) readBackHold(holdNo string) (*money.FreezeWalletResp, error) {
	h, err := l.svcCtx.WalletHoldModel.FindOneByHoldNo(l.ctx, holdNo)
	if err != nil {
		l.Errorf("FreezeWallet readBackHold failed, holdNo: %s, err: %v", holdNo, err)
		return nil, err
	}
	return &money.FreezeWalletResp{HoldNo: h.HoldNo, Frozen: utils.FenToYuan(h.Amount)}, nil
}
