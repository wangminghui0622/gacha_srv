package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"

	"gacha_srv/app/dtm/client/dtmgrpc"
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

type ReleaseQuoteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseQuoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseQuoteLogic {
	return &ReleaseQuoteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 10/30 完善：补 USED 孤儿核销清扫（CommitQuote 已提交但订单未 PAID 时回退）。

// ReleaseQuote SAGA 分支 03 补偿：barrier 事务内 FOR UPDATE 锁报价行 + 券行 + 资格行。
//
//	quote LOCKED/OPEN→RELEASED；券 LOCKED（locked_by=本报价）→AVAILABLE；资格 LOCKED（同）→AVAILABLE。
//
// 幂等：RELEASED/EXPIRED 直接成功；USED → 孤儿核销回退（07 未 PAID 时 CommitQuote 已提交），券/资格 USED 且 locked_by=本报价 → AVAILABLE。
//
// 双调用形态：抽盒 SAGA 由 DTM 直调（带 trans info 走 barrier）；
// play.Release<玩法> 分支内 sub-call（无 trans info，退化为普通事务，业务幂等兜底）。
func (l *ReleaseQuoteLogic) ReleaseQuote(in *marketing.ReleaseQuoteReq) (*marketing.ReleaseQuoteResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 双调用形态：SAGA 分支 03 补偿被 DTM 直调时带 trans info 走 barrier；
	// play.Release<玩法> 分支内 sub-call 无 trans info，退化为普通事务
	//（幂等由 FOR UPDATE + 状态校验兜底：RELEASED/EXPIRED 直返，USED 孤儿回退）。
	barrier, err := dtmgrpc.BarrierFromGrpcOrNil(l.ctx)
	if err != nil {
		l.Errorf("ReleaseQuote get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	statusOut := ""
	businessRan := false
	busi := func(db *gorm.DB) error {
		businessRan = true

		var quote gamemodelnc.QuoteLocks
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("quote_token = ?", in.QuoteToken).First(&quote).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价不存在", errorx.CodeQuoteNotFound))
		}
		if err != nil {
			return err
		}
		// USED=孤儿核销（10/30）：03 补偿只在 07 未捕获成功时到来，此刻 USED 必为
		// 「CommitQuote 已提交、订单未 PAID」的孤儿（响应丢失 + 重试耗尽），需回退；
		// RELEASED/EXPIRED：幂等无操作。
		if quote.Status == quoteUsed {
			if err := l.uncommitOrphanQuote(db, &quote); err != nil {
				return err
			}
			statusOut = "RELEASED"
			return nil
		}
		if quote.Status == quoteReleased || quote.Status == quoteExpired {
			statusOut = quoteStatusName(quote.Status)
			l.Infof("ReleaseQuote idempotent: status %s, quoteToken: %s", statusOut, in.QuoteToken)
			return nil
		}

		// 回退券：LOCKED 且 locked_by=本报价 → AVAILABLE（USED 不回退，上面已拦）
		if quote.CouponNo != "" {
			var coupon gamemodelnc.UserCoupons
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("coupon_no = ?", quote.CouponNo).First(&coupon).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil && coupon.Status == couponLocked && coupon.LockedBy == in.QuoteToken {
				if err := db.Model(&gamemodelnc.UserCoupons{}).
					Where("coupon_no = ? AND status = ?", quote.CouponNo, couponLocked).
					Updates(map[string]interface{}{"status": couponAvailable, "locked_by": ""}).Error; err != nil {
					return err
				}
			}
		}

		// 回退资格：LOCKED 且 locked_by=本报价 → AVAILABLE（USED 不回退，上面已拦）
		if quote.EntitlementNo != "" {
			var ent gamemodelnc.CampaignBoxEntitlements
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("entitlement_no = ?", quote.EntitlementNo).First(&ent).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil && ent.Status == entitlementLocked && ent.LockedBy == in.QuoteToken {
				if err := db.Model(&gamemodelnc.CampaignBoxEntitlements{}).
					Where("entitlement_no = ? AND status = ?", quote.EntitlementNo, entitlementLocked).
					Updates(map[string]interface{}{"status": entitlementAvailable, "locked_by": ""}).Error; err != nil {
					return err
				}
			}
		}

		// 报价 → RELEASED（盲更新，幂等）
		if err := db.Model(&gamemodelnc.QuoteLocks{}).
			Where("quote_token = ? AND status IN ?", in.QuoteToken, []int64{quoteOpen, quoteLocked}).
			Update("status", quoteReleased).Error; err != nil {
			return err
		}
		statusOut = "RELEASED"
		return nil
	}
	if barrier != nil {
		rawDB, err := l.svcCtx.DB.DB()
		if err != nil {
			l.Errorf("ReleaseQuote get raw db failed, err: %v", err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
			return busi(txutil.WithTx(l.ctx, l.svcCtx.DB, tx))
		})
	} else {
		err = l.svcCtx.DB.Transaction(busi)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("ReleaseQuote aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseQuote failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读库如实返回
		q, qe := l.svcCtx.QuoteModel.FindOneByQuoteToken(l.ctx, in.QuoteToken)
		if qe == nil {
			statusOut = quoteStatusName(q.Status)
		} else if !errors.Is(qe, gamemodelnc.ErrNotFound) {
			return nil, status.Error(codes.Internal, qe.Error())
		}
	}
	l.Infof("ReleaseQuote success, quoteToken: %s, status: %s", in.QuoteToken, statusOut)
	return &marketing.ReleaseQuoteResp{QuoteToken: in.QuoteToken, Status: statusOut}, nil
}

// 10/30 uncommitOrphanQuote 孤儿核销清扫：把 USED 的报价/券/资格回退为可再释放状态。
// 触发：07 捕获分支 CommitQuote 已提交、订单未 PAID（响应丢失 + 重试耗尽），
// 03 补偿到来时 quote=USED。locked_by 仍指向本报价，作归属校验防误回退他人资源。
func (l *ReleaseQuoteLogic) uncommitOrphanQuote(db *gorm.DB, quote *gamemodelnc.QuoteLocks) error {
	if quote.CouponNo != "" {
		if err := db.Model(&gamemodelnc.UserCoupons{}).
			Where("coupon_no = ? AND status = ? AND locked_by = ?", quote.CouponNo, couponUsed, quote.QuoteToken).
			Updates(map[string]interface{}{"status": couponAvailable, "locked_by": ""}).Error; err != nil {
			return err
		}
	}
	if quote.EntitlementNo != "" {
		if err := db.Model(&gamemodelnc.CampaignBoxEntitlements{}).
			Where("entitlement_no = ? AND status = ? AND locked_by = ?", quote.EntitlementNo, entitlementUsed, quote.QuoteToken).
			Updates(map[string]interface{}{"status": entitlementAvailable, "locked_by": ""}).Error; err != nil {
			return err
		}
	}
	return db.Model(&gamemodelnc.QuoteLocks{}).
		Where("quote_token = ? AND status = ?", quote.QuoteToken, quoteUsed).
		Update("status", quoteReleased).Error
}
