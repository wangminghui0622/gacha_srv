package logic

import (
	"context"
	"errors"

	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/model_nocache/gacha_money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostHoldLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPostHoldLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PostHoldLogic {
	return &PostHoldLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PostHold 捕获入账（非 DTM 分支，被 CaptureWalletPayment SAGA 分支 07 内嵌直调）：
// PENDING→POSTED 机械状态迁移；POSTED 幂等成功，VOIDED/EXPIRED→CodeHoldNotFound 2003。
// 返回 CodeError 业务码，调用方包装为 codes.Aborted；捕获期业务失败已前置
// （分支 04 冻结成功即足额），此处 Aborted 属防御性语义。
func (l *PostHoldLogic) PostHold(in *money.PostHoldReq) (*money.PostHoldResp, error) {
	if in.HoldNo == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "holdNo不能为空")
	}

	var idempotentHold *gacha_money.WalletHolds
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
		case holdStatusPosted:
			idempotentHold = &h
			return nil // 幂等成功：已入账
		case holdStatusVoided, holdStatusExpired:
			return errorx.NewCodeError(errorx.CodeHoldNotFound, "冻结已释放或过期")
		}
		// PENDING→POSTED 盲更新（行已锁，WHERE status 过滤属防御性编程）
		result := tx.Model(&gacha_money.WalletHolds{}).
			Where("hold_no = ? AND status = ?", in.HoldNo, holdStatusPending).
			Update("status", holdStatusPosted)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errorx.NewCodeError(errorx.CodeHoldNotFound, "冻结已释放或过期")
		}
		// held 核减：钱从冻结池划出，可用余额不再变化
		return tx.Model(&gacha_money.WalletAccounts{}).
			Where("user_id = ? AND wallet_kind = ?", h.UserId, h.WalletKind).
			UpdateColumn("held", gorm.Expr("held - ?", h.Amount)).Error
	})
	if err != nil {
		if errorx.IsCodeError(err) {
			l.Errorf("PostHold aborted, holdNo: %s, reason: %s", in.HoldNo, err.Error())
			return nil, err
		}
		l.Errorf("PostHold failed, holdNo: %s, err: %v", in.HoldNo, err)
		return nil, err
	}
	if idempotentHold != nil {
		l.Infof("PostHold idempotent success, holdNo: %s", in.HoldNo)
	} else {
		l.Infof("PostHold success, holdNo: %s", in.HoldNo)
	}
	return &money.PostHoldResp{Status: int32(holdStatusPosted)}, nil
}
