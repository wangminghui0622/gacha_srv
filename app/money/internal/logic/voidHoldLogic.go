package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VoidHoldLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVoidHoldLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VoidHoldLogic {
	return &VoidHoldLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VoidHold 释放冻结（非 DTM 分支，被 CancelPayment 撤单内嵌直调）：
// PENDING→VOIDED 机械状态迁移；VOIDED/EXPIRED 幂等成功；
// POSTED→已入账不可逆（07 PostHold 已真扣款，补偿到来说明 07 在 PostHold 之后失败，
// 如 CommitQuote 失败/响应丢失）——返回成功 + Errorf 告警：钱不退，DTM 事务正常
// 完结不再无限重试楔死；冻结保持 POSTED 待对账/人工处理。
// 「退款成功则奖不追、发奖已落库则不退钱」的对称语义由 CancelPayment 的
// 发奖落库前置检查（CheckPrizesGranted/CheckGrantsByRequestId）保证。
// 返回 CodeError 业务码（hold 不存在 → 2003），调用方按码分流。
func (l *VoidHoldLogic) VoidHold(in *money.VoidHoldReq) (*money.VoidHoldResp, error) {
	if in.HoldNo == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "holdNo不能为空")
	}

	var idempotentHold *gacha_money.WalletHolds
	postedHold := false
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var h gacha_money.WalletHolds
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("hold_no = ?", in.HoldNo).First(&h).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorx.NewCodeError(errorx.CodeHoldNotFound, "冻结单不存在")
			}
			return err
		}
		// 防串单：业务类型/单号不匹配直接拒绝
		if h.BizType != in.BizType || h.BizId != in.BizId {
			return errorx.NewCodeError(errorx.CodeHoldBizMismatch, "冻结单业务信息不匹配")
		}
		switch h.Status {
		case holdStatusVoided, holdStatusExpired:
			idempotentHold = &h
			return nil // 幂等成功：已释放（EXPIRED 无过期回收工，视同已释放，仅防御路径）
		case holdStatusPosted:
			// 已入账冻结不可释放：不可逆真扣款，不退钱（返回成功让 SAGA 补偿正常完结）
			idempotentHold = &h
			postedHold = true
			return nil
		}
		// PENDING→VOIDED 盲更新（行已锁，WHERE status 过滤属防御性编程）
		result := tx.Model(&gacha_money.WalletHolds{}).
			Where("hold_no = ? AND status = ?", in.HoldNo, holdStatusPending).
			Update("status", holdStatusVoided)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errorx.NewCodeError(errorx.CodeHoldNotFound, "冻结状态已变更")
		}
		// 解冻回账：available+、held-（退款目标取行所有者 h.UserId/h.WalletKind，不信入参）
		if err := tx.Model(&gacha_money.WalletAccounts{}).
			Where("user_id = ? AND wallet_kind = ?", h.UserId, h.WalletKind).
			UpdateColumns(map[string]interface{}{
				"available": gorm.Expr("available + ?", h.Amount),
				"held":      gorm.Expr("held - ?", h.Amount),
			}).Error; err != nil {
			return err
		}
		// 解冻流水（log_no="v_"+bizId 确定性，重试幂等）
		return l.svcCtx.WalletLedgerModel.Insert(l.ctx, tx, &gacha_money.WalletLedgers{
			LogNo:      "v_" + in.BizId,
			UserId:     h.UserId,
			WalletKind: h.WalletKind,
			Amount:     h.Amount,
			BizType:    "unfreeze",
			BizId:      in.BizId,
			HoldNo:     in.HoldNo,
			Status:     "success",
			CreatedAt:  time.Now(),
		})
	})
	if err != nil {
		if errorx.IsCodeError(err) {
			l.Errorf("VoidHold aborted, holdNo: %s, reason: %s", in.HoldNo, err.Error())
			return nil, err
		}
		l.Errorf("VoidHold failed, holdNo: %s, err: %v", in.HoldNo, err)
		return nil, err
	}
	if postedHold {
		// 告警：已入账冻结不可释放，钱保持扣除（用户不能白拿奖品），冻结保持可恢复待对账
		l.Errorf("VoidHold skipped: hold already posted, keep money deducted, holdNo: %s, bizType: %s, bizId: %s",
			in.HoldNo, in.BizType, in.BizId)
	} else if idempotentHold != nil {
		l.Infof("VoidHold idempotent success, holdNo: %s", in.HoldNo)
	} else {
		l.Infof("VoidHold success, holdNo: %s", in.HoldNo)
	}
	return &money.VoidHoldResp{Status: int32(holdStatusVoided)}, nil
}
