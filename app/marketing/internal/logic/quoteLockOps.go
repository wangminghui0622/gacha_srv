package logic

import (
	"errors"
	"fmt"
	"time"

	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// acquireQuoteLock 报价+券+资格锁定（LockQuote / LockDigBox 共用，必须在 barrier 事务内调用）：
//   - FOR UPDATE 锁报价行，校验状态与过期（LOCKED → alreadyLocked=true 幂等返回；
//     USED/RELEASED/EXPIRED → Aborted；OPEN 过期 → Aborted）
//   - 有券时 FOR UPDATE 锁券行：AVAILABLE→LOCKED（locked_by=quoteToken）；已被本报价锁 → 跳过
//   - 有资格时 FOR UPDATE 锁资格行：AVAILABLE→LOCKED（locked_by=quoteToken）；已被本报价锁 → 跳过
//
// 调用方拿到 alreadyLocked=false 后继续业务，并在事务末尾盲更新 quote OPEN→LOCKED。
func acquireQuoteLock(db *gorm.DB, quoteToken string) (quote *gamemodelnc.QuoteLocks, alreadyLocked bool, err error) {
	var q gamemodelnc.QuoteLocks
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("quote_token = ?", quoteToken).First(&q).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价不存在", errorx.CodeQuoteNotFound))
		}
		return nil, false, err
	}
	switch q.Status {
	case quoteLocked:
		// 幂等：本报价此前已锁定（DTM 重试/响应丢失）→ 调用方跳过业务
		return &q, true, nil
	case quoteUsed, quoteReleased:
		return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价状态不满足锁定", errorx.CodeQuoteStateInvalid))
	case quoteExpired:
		return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价已过期", errorx.CodeQuoteNotFound))
	}
	// OPEN：过期兜底（状态未推进到 EXPIRED 时按时间判）
	if time.Now().After(q.ExpiresAt) {
		return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价已过期", errorx.CodeQuoteNotFound))
	}

	// 锁券（有券才锁；AVAILABLE→LOCKED，本报价已锁则幂等跳过）
	if q.CouponNo != "" {
		var coupon gamemodelnc.UserCoupons
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("coupon_no = ?", q.CouponNo).First(&coupon).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券不存在", errorx.CodeCouponUnavailable))
			}
			return nil, false, err
		}
		lockCoupon := false
		switch coupon.Status {
		case couponAvailable:
			lockCoupon = true
		case couponLocked:
			if coupon.LockedBy != quoteToken {
				return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券已被其他报价锁定", errorx.CodeCouponUnavailable))
			}
			// 幂等：本报价已锁该券 → 跳过更新
		default:
			return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券状态不可用", errorx.CodeCouponUnavailable))
		}
		if lockCoupon {
			if err := db.Model(&gamemodelnc.UserCoupons{}).Where("coupon_no = ? AND status = ?", q.CouponNo, couponAvailable).Updates(map[string]interface{}{"status": couponLocked, "locked_by": quoteToken}).Error; err != nil {
				return nil, false, err
			}
		}
	}

	// 锁资格（有资格才锁；AVAILABLE→LOCKED，本报价已锁则幂等跳过）
	if q.EntitlementNo != "" {
		var ent gamemodelnc.CampaignBoxEntitlements
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("entitlement_no = ?", q.EntitlementNo).First(&ent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=资格不存在", errorx.CodeEntitlementNotFound))
			}
			return nil, false, err
		}
		lockEnt := false
		switch ent.Status {
		case entitlementAvailable:
			lockEnt = true
		case entitlementLocked:
			if ent.LockedBy != quoteToken {
				return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=资格已被其他报价锁定", errorx.CodeEntitlementUnavail))
			}
			// 幂等：本报价已锁该资格 → 跳过更新
		default:
			return nil, false, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=资格状态不可用", errorx.CodeEntitlementUnavail))
		}
		if lockEnt {
			if err := db.Model(&gamemodelnc.CampaignBoxEntitlements{}).Where("entitlement_no = ? AND status = ?", q.EntitlementNo, entitlementAvailable).
				Updates(map[string]interface{}{"status": entitlementLocked, "locked_by": quoteToken}).Error; err != nil {
				return nil, false, err
			}
		}
	}
	return &q, false, nil
}
