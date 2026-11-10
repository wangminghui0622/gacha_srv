package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/app/money/moneyClient"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const loyaltyWalletKind = 4 // LOYALTY_POINT（与 user 域 wallet_kind 对齐）

type QuoteCheckoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQuoteCheckoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QuoteCheckoutLogic {
	return &QuoteCheckoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// QuoteCheckout 报价试算：服务端算价（gross 来自网关取 draw 的箱价），客户端只传选择。
// 券/积分/活动宝箱资格三者互斥；资格全抵（couponDiscountFen=gross → payable=0）。
// 积分抵扣受 loyalty_offset_config 上限与可用余额截断。
// 资格只在此校验（存在/归属/AVAILABLE/箱型匹配），真正的状态推进在 LockQuote 屏障事务内。
// 幂等：uk request_id，撞键/重发读回原报价（任意状态都返回，Submit 重试同 gid 复用）。
//
// 错误语义：业务失败 → codes.Aborted（含 code=300x msg=...）；系统错误 → codes.Internal。
func (l *QuoteCheckoutLogic) QuoteCheckout(in *marketing.QuoteCheckoutReq) (*marketing.QuoteCheckoutResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.BoxId <= 0 || in.GrossFen <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}
	quoteToken := "q_" + in.RequestId

	// ① 幂等：撞 uk request_id 读回原报价（先查后插 + 插入撞键双保险）；
	//    低c 修复：读回校验归属（quote.user_id == 请求者），防换人重放他人报价单
	if existing, err := l.svcCtx.QuoteModel.FindOneByRequestId(l.ctx, in.RequestId); err == nil {
		if existing.UserId != in.UserId {
			l.Errorf("QuoteCheckout read-back owner mismatch, requestId: %s, quote user: %d, requester: %d",
				in.RequestId, existing.UserId, in.UserId)
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价单不属于当前用户", errorx.CodeQuoteNotFound))
		}
		l.Infof("QuoteCheckout idempotent read-back, requestId: %s, status: %d", in.RequestId, existing.Status)
		return buildQuoteResp(existing), nil
	} else if !errors.Is(err, gamemodelnc.ErrNotFound) {
		l.Errorf("QuoteCheckout read-back query failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ② 券/积分/资格互斥
	if in.UserCouponId > 0 && in.UseLoyaltyOffset {
		return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券与积分抵扣互斥，只能选其一", errorx.CodeCouponUnavailable))
	}
	if in.EntitlementNo != "" && (in.UserCouponId > 0 || in.UseLoyaltyOffset) {
		return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=资格与券/积分抵扣互斥，只能选其一", errorx.CodeEntitlementUnavail))
	}

	// ③ 活动宝箱资格校验（归属/状态/箱型；全抵后 payable=0，真正的锁定在 LockQuote 屏障内）
	if in.EntitlementNo != "" {
		ent, err := l.svcCtx.EntitlementModel.FindOneByEntitlementNo(l.ctx, in.EntitlementNo)
		if errors.Is(err, gamemodelnc.ErrNotFound) {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=资格不存在", errorx.CodeEntitlementNotFound))
		}
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if ent.UserId != in.UserId {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=资格不属于当前用户", errorx.CodeEntitlementUnavail))
		}
		if ent.Status != entitlementAvailable {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=资格状态不可用", errorx.CodeEntitlementUnavail))
		}
		if ent.BoxId != in.BoxId {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=资格与所选宝箱不符", errorx.CodeEntitlementUnavail))
		}
	}

	// ④ 券抵扣计算（服务端按模板规则算）
	var couponNo string
	var couponDiscountFen int64
	if in.UserCouponId > 0 {
		coupon, err := l.svcCtx.CouponModel.FindOne(l.ctx, in.UserCouponId)
		if errors.Is(err, gamemodelnc.ErrNotFound) {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券不存在", errorx.CodeCouponUnavailable))
		}
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if coupon.UserId != in.UserId {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券不属于当前用户", errorx.CodeCouponUnavailable))
		}
		if coupon.Status != couponAvailable {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券状态不可用", errorx.CodeCouponUnavailable))
		}
		tpl, err := l.svcCtx.TemplateModel.FindOneByTemplateNo(l.ctx, coupon.TemplateNo)
		if errors.Is(err, gamemodel.ErrNotFound) || (err == nil && tpl.Status != templatePublished) {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券模板不存在或未发布", errorx.CodeCouponTemplateNotFound))
		}
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if in.GrossFen < tpl.MinGross {
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=订单未达券使用门槛", errorx.CodeCouponUnavailable))
		}
		switch tpl.Kind {
		case 1: // FIXED_MINOR：直减
			couponDiscountFen = minInt64(tpl.Value, in.GrossFen)
		case 2: // PERCENT_BPS：折扣（万分比）
			couponDiscountFen = minInt64(in.GrossFen*tpl.Value/10000, in.GrossFen)
		case 3: // FREE：免单
			couponDiscountFen = in.GrossFen
		default: // 4 CUSTOM_MINOR / 5 DRAW_REWARD 后置
			return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=该券类型暂不支持", errorx.CodeCouponUnavailable))
		}
		couponNo = coupon.CouponNo
	}
	// 资格全抵：entitlement 走券抵扣列（couponDiscountFen=gross → payable=0），券号列留空
	if in.EntitlementNo != "" {
		couponDiscountFen = in.GrossFen
	}

	// ⑤ 积分抵扣计算（读用户域积分余额 + 全局抵扣配置）
	var loyaltyPointsDebit, loyaltyFen, loyaltyPointBalance, loyaltyOffsetRate, loyaltyMaxFen int64
	if in.UseLoyaltyOffset {
		cfg, err := l.svcCtx.LoyaltyCfgModel.FindOne(l.ctx, 1)
		if err != nil {
			l.Errorf("QuoteCheckout loyalty config missing, err: %v", err)
			return nil, status.Error(codes.Internal, "积分抵扣配置缺失")
		}
		rate := int64(cfg.PointsPerYuan)
		if rate <= 0 || cfg.Status != 1 {
			return nil, status.Error(codes.Internal, "积分抵扣配置未启用")
		}
		wallet, err := l.svcCtx.MoneyRpc.GetWallet(l.ctx, &moneyClient.GetWalletReq{UserId: in.UserId})
		if err != nil {
			l.Errorf("QuoteCheckout GetWallet failed, userId: %d, err: %v", in.UserId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		points := int64(0)
		for _, k := range wallet.Kinds {
			if k.Kind == loyaltyWalletKind {
				points, err = utils.YuanToFen(k.Available)
				if err != nil {
					return nil, status.Error(codes.Internal, "积分余额解析失败")
				}
			}
		}
		maxFen := in.GrossFen * int64(cfg.MaxRatioBps) / 10000 // 上限=原价×70%
		pointFen := points * 100 / rate                        // 1 积分=100/rate 分（10 积分=1 元 → 1 积分=10 分）
		loyaltyFen = minInt64(maxFen, pointFen)
		loyaltyPointsDebit = loyaltyFen * rate / 100
		loyaltyPointBalance = points
		loyaltyOffsetRate = rate
		loyaltyMaxFen = maxFen
	}

	// ⑥ 组装实付（payable = cash + loyalty = gross - coupon_discount：
	// 券/资格折扣减总额、积分抵扣是支付拆分而非折扣；互斥故 cash ≥ 0 恒成立）
	payableFen := in.GrossFen - couponDiscountFen
	if payableFen < 0 {
		payableFen = 0
	}
	cashFen := payableFen - loyaltyFen
	if cashFen < 0 {
		cashFen = 0
	}

	// ⑦ 落 OPEN 报价锁（15 分钟 TTL；并发同 gid 撞键 → 读回返回）
	now := time.Now()
	row := &gamemodelnc.QuoteLocks{
		QuoteToken:         quoteToken,
		RequestId:          in.RequestId,
		UserId:             in.UserId,
		BoxId:              in.BoxId,
		GrossFen:           in.GrossFen,
		CouponNo:           couponNo,
		CouponDiscountFen:  couponDiscountFen,
		EntitlementNo:      in.EntitlementNo,
		LoyaltyPointsDebit: loyaltyPointsDebit,
		LoyaltyFen:         loyaltyFen,
		PayableFen:         payableFen,
		CashFen:            cashFen,
		Status:             quoteOpen,
		ExpiresAt:          now.Add(15 * time.Minute),
	}
	if err := l.svcCtx.QuoteModel.Insert(l.ctx, l.svcCtx.DB, row); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 低c 修复：插入撞键读回同样校验归属，防换人重放他人报价单
			if existing, qe := l.svcCtx.QuoteModel.FindOneByRequestId(l.ctx, in.RequestId); qe == nil {
				if existing.UserId != in.UserId {
					l.Errorf("QuoteCheckout duplicate key owner mismatch, requestId: %s, quote user: %d, requester: %d",
						in.RequestId, existing.UserId, in.UserId)
					return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=报价单不属于当前用户", errorx.CodeQuoteNotFound))
				}
				l.Infof("QuoteCheckout duplicate key, read-back, requestId: %s", in.RequestId)
				return buildQuoteResp(existing), nil
			}
		}
		l.Errorf("QuoteCheckout insert failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := buildQuoteResp(row)
	resp.LoyaltyPointBalance = loyaltyPointBalance
	resp.LoyaltyOffsetRate = loyaltyOffsetRate
	resp.LoyaltyMaxFen = loyaltyMaxFen
	l.Infof("QuoteCheckout ok, requestId: %s, gross: %d, coupon: %d, loyalty: %d, payable: %d, entitlement: %s",
		in.RequestId, in.GrossFen, couponDiscountFen, loyaltyFen, payableFen, in.EntitlementNo)
	return resp, nil
}
