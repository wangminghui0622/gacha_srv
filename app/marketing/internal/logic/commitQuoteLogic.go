package logic

import (
	"context"
	"errors"
	"fmt"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommitQuoteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCommitQuoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CommitQuoteLogic {
	return &CommitQuoteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CommitQuote SAGA 分支 07 捕获的 sub-call（payment 调用）：报价 LOCKED→USED，
// 同事务核销券（LOCKED→USED）与资格（LOCKED→USED）。
// 无 barrier（非 DTM 直接分支）：幂等靠状态机——USED 直接成功。
// 自身独立事务 + FOR UPDATE 锁报价行，防与 03 补偿（ReleaseQuote）并发交错。
func (l *CommitQuoteLogic) CommitQuote(in *marketing.CommitQuoteReq) (*marketing.CommitQuoteResp, error) {
	if in.QuoteToken == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=报价令牌不能为空")
	}

	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var quote gamemodelnc.QuoteLocks
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("quote_token = ?", in.QuoteToken).First(&quote).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价不存在", errorx.CodeQuoteNotFound))
		}
		if err != nil {
			return err
		}
		switch quote.Status {
		case quoteUsed:
			l.Infof("CommitQuote idempotent: already used, quoteToken: %s", in.QuoteToken)
			return nil
		case quoteLocked:
			// LOCKED→USED（下方盲更新；券同事务核销）
		default:
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价状态不满足核销", errorx.CodeQuoteStateInvalid))
		}
		if err := tx.Model(&gamemodelnc.QuoteLocks{}).
			Where("quote_token = ? AND status = ?", in.QuoteToken, quoteLocked).
			Update("status", quoteUsed).Error; err != nil {
			return err
		}
		// 核销券：LOCKED（本报价锁）→ USED（幂等：USED 不动）
		if quote.CouponNo != "" {
			var coupon gamemodelnc.UserCoupons
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("coupon_no = ?", quote.CouponNo).First(&coupon).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil && coupon.Status == couponLocked && coupon.LockedBy == in.QuoteToken {
				if err := tx.Model(&gamemodelnc.UserCoupons{}).
					Where("coupon_no = ? AND status = ?", quote.CouponNo, couponLocked).
					Update("status", couponUsed).Error; err != nil {
					return err
				}
			}
		}
		// 核销资格：LOCKED（本报价锁）→ USED（幂等：USED 不动）
		if quote.EntitlementNo != "" {
			var ent gamemodelnc.CampaignBoxEntitlements
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("entitlement_no = ?", quote.EntitlementNo).First(&ent).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil && ent.Status == entitlementLocked && ent.LockedBy == in.QuoteToken {
				if err := tx.Model(&gamemodelnc.CampaignBoxEntitlements{}).
					Where("entitlement_no = ? AND status = ?", quote.EntitlementNo, entitlementLocked).
					Update("status", entitlementUsed).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("CommitQuote aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("CommitQuote failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("CommitQuote success, quoteToken: %s", in.QuoteToken)
	return &marketing.CommitQuoteResp{QuoteToken: in.QuoteToken, Status: "USED"}, nil
}
