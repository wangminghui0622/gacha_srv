package logic

import (
	"context"
	"encoding/json"
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
	"gorm.io/gorm"
)

// ══════════════ M11 campaign 活动域公共操作 ═══════════════
// 发奖无 outbox/Kafka（计划 D3）：本地事务先落 participation + campaign_prize_grants（PENDING），
// 事务外逐条外部幂等发放（CreditWallet requestId=grant_no / 券 "uc_"+grant_no / 资格 "ce_"+grant_no），
// 失败 Internal 由客户端同 Idempotency-Key 重试，重放只补发 PENDING。

// 活动类型（campaigns.kind；转盘 13 对齐 server_v2 CampaignKind.PRIZE_WHEEL，M14d 起）
const (
	campaignSignIn     int64 = 1  // 签到
	campaignRedeemCode int64 = 2  // 兑换码
	campaignWheel      int64 = 13 // 转盘
)

// 奖品类型（campaign_prize_grants.prize_kind / 转盘槽 prize_kind）
const (
	prizeCash     int64 = 1 // 现金（CreditWallet kind=1）
	prizePoints   int64 = 2 // 积分（CreditWallet kind=4）
	prizeCoupon   int64 = 3 // 券（本地插入 "uc_"+grant_no）
	prizeGachaBox int64 = 4 // 活动宝箱资格（本地插入 "ce_"+grant_no）
)

// 奖品发放流水状态
const (
	grantPending int64 = 0
	grantGranted int64 = 1
)

// 活动宝箱资格状态（campaign_box_entitlements.status）
const (
	entitlementAvailable int64 = 1
	entitlementLocked    int64 = 2
	entitlementUsed      int64 = 3
)

// 信用钱包类型（与 user 域 wallet_kind 对齐；积分走 LOYALTY_POINT，1 积分=1 分存储）
const (
	walletCash         int32 = 1
	walletLoyaltyPoint int32 = 4
)

// prizeCfg 配置/槽位里的奖品条目（config_json / prize_config 数组元素）
type prizeCfg struct {
	Kind             int64  `json:"kind"`                       // 1 CASH 2 POINTS 3 COUPON 4 GACHA_BOX
	CashFen          int64  `json:"cashFen"`                    // 现金金额（分）
	Points           int64  `json:"points"`                     // 积分数
	CouponTemplateNo string `json:"couponTemplateNo,omitempty"` // 券模板单号
	BoxId            int64  `json:"boxId"`                      // 宝箱ID（kind=4）
}

// signInDayCfg 签到阶梯某一天（rewards 数组元素）
type signInDayCfg struct {
	Prizes []prizeCfg `json:"prizes"`
}

// signInConfig 签到活动 config_json
type signInConfig struct {
	Rewards []signInDayCfg `json:"rewards"`
}

// wheelConfig 转盘活动 config_json
type wheelConfig struct {
	DailyLimit int64 `json:"dailyLimit"`
}

// participateResult 参与结果（campaign_participations.result_json，重放原样返回）
type participateResult struct {
	Streak    int64                  `json:"streak"`    // 签到：本次后连续天数
	SpinsLeft int64                  `json:"spinsLeft"` // 转盘：本次后今日剩余次数
	Prizes    []*marketing.PrizeItem `json:"prizes"`    // 本次获得奖品
}

// parseConfig 解析活动 config_json（按 kind 解析，失败返回 5007）
func parseCampaignConfig(kind int64, configJson string) (*signInConfig, *wheelConfig, error) {
	switch kind {
	case campaignSignIn:
		var cfg signInConfig
		if err := json.Unmarshal([]byte(configJson), &cfg); err != nil || len(cfg.Rewards) == 0 {
			return nil, nil, errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "签到配置无效")
		}
		return &cfg, nil, nil
	case campaignWheel:
		var cfg wheelConfig
		if err := json.Unmarshal([]byte(configJson), &cfg); err != nil {
			return nil, nil, errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "转盘配置无效")
		}
		if cfg.DailyLimit <= 0 {
			cfg.DailyLimit = 1
		}
		return nil, &cfg, nil
	default:
		return nil, nil, nil
	}
}

// parsePrizeConfig 解析兑换码 prize_config 数组
func parsePrizeConfig(prizeConfig string) ([]prizeCfg, error) {
	var prizes []prizeCfg
	if err := json.Unmarshal([]byte(prizeConfig), &prizes); err != nil || len(prizes) == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "兑换码奖品配置无效")
	}
	return prizes, nil
}

// localDate 服务本地日期串（DSN loc=Local，Asia/Shanghai）
func localDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func localToday() string { return localDate(time.Now()) }

func localYesterday() string { return localDate(time.Now().AddDate(0, 0, -1)) }

// localTodayTime 服务本地今日零点（DATE 列查询参数用）
func localTodayTime() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// isNotFound 模型查询未命中（cache 与 nocache 两套 ErrNotFound 都认）
func isNotFound(err error) bool {
	return errors.Is(err, gamemodelnc.ErrNotFound) || errors.Is(err, gamemodel.ErrNotFound)
}

// inWindow 活动是否在窗口内
func inWindow(startAt, endAt time.Time) bool {
	now := time.Now()
	return !now.Before(startAt) && now.Before(endAt)
}

// prizeItemOf 配置奖品条目 → 响应 PrizeItem（现金金额转元字符串；券名尽力解析，失败给空名）
func prizeItemOf(ctx context.Context, svcCtx *svc.ServiceContext, p prizeCfg) *marketing.PrizeItem {
	item := &marketing.PrizeItem{Kind: p.Kind, Points: p.Points, CouponTemplateNo: p.CouponTemplateNo, BoxId: p.BoxId}
	if p.Kind == prizeCash {
		item.Amount = utils.FenToYuan(p.CashFen)
	}
	if p.Kind == prizeCoupon && p.CouponTemplateNo != "" {
		if tpl, err := svcCtx.TemplateModel.FindOneByTemplateNo(ctx, p.CouponTemplateNo); err == nil {
			item.CouponName = tpl.Name
		}
	}
	return item
}

// prizeItemOfGrant 发放流水行 → 响应 PrizeItem（重放结果组装）
func prizeItemOfGrant(ctx context.Context, svcCtx *svc.ServiceContext, g *gamemodelnc.CampaignPrizeGrants) *marketing.PrizeItem {
	return prizeItemOf(ctx, svcCtx, prizeCfg{
		Kind:             g.PrizeKind,
		CashFen:          g.AmountFen,
		Points:           g.PointsAmount,
		CouponTemplateNo: g.CouponTemplateNo,
		BoxId:            g.BoxId,
	})
}

// grantPrizesPending 补发 idem_key 下全部 PENDING 流水（幂等：grant_no 确定性派生）。
// 任一发放失败即返回错误（客户端同 Idempotency-Key 重试后重放补发）。
func grantPrizesPending(ctx context.Context, svcCtx *svc.ServiceContext, idemKey string) error {
	var grants []*gamemodelnc.CampaignPrizeGrants
	if err := svcCtx.DB.Where("idem_key = ? AND status = ?", idemKey, grantPending).
		Order("id").Find(&grants).Error; err != nil {
		return err
	}
	for _, g := range grants {
		if err := grantOnePrize(ctx, svcCtx, g); err != nil {
			return err
		}
		if err := svcCtx.DB.Model(&gamemodelnc.CampaignPrizeGrants{}).
			Where("grant_no = ? AND status = ?", g.GrantNo, grantPending).
			Updates(map[string]interface{}{"status": grantGranted, "granted_at": time.Now()}).Error; err != nil {
			return err
		}
	}
	return nil
}

// grantOnePrize 单条外部发放（全 uk 幂等：重试/并发重复直接收敛，不重复发放）
func grantOnePrize(ctx context.Context, svcCtx *svc.ServiceContext, g *gamemodelnc.CampaignPrizeGrants) error {
	switch g.PrizeKind {
	case prizeCash:
		if _, err := svcCtx.MoneyRpc.CreditWallet(ctx, &moneyClient.CreditWalletReq{
			UserId:    g.UserId,
			Kind:      walletCash,
			Amount:    utils.FenToYuan(g.AmountFen),
			RequestId: g.GrantNo,
		}); err != nil {
			return err
		}
	case prizePoints:
		if _, err := svcCtx.MoneyRpc.CreditWallet(ctx, &moneyClient.CreditWalletReq{
			UserId:    g.UserId,
			Kind:      walletLoyaltyPoint,
			Amount:    utils.FenToYuan(g.PointsAmount),
			RequestId: g.GrantNo,
		}); err != nil {
			return err
		}
	case prizeCoupon:
		// 本地插入用户券（coupon_no="uc_"+grant_no；uk 撞键=已发过，幂等跳过）
		row := &gamemodelnc.UserCoupons{
			CouponNo:   "uc_" + g.GrantNo,
			UserId:     g.UserId,
			TemplateNo: g.CouponTemplateNo,
			Status:     couponAvailable,
			LockedBy:   "",
			ExpiresAt:  nil,
		}
		if err := svcCtx.CouponModel.Insert(ctx, svcCtx.DB, row); err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	case prizeGachaBox:
		// 本地插入活动宝箱资格（entitlement_no="ce_"+grant_no；uk 撞键=已发过）
		var participation gamemodelnc.CampaignParticipations
		if err := svcCtx.DB.Where("id = ?", g.ParticipationId).First(&participation).Error; err != nil {
			return err
		}
		row := &gamemodelnc.CampaignBoxEntitlements{
			EntitlementNo: "ce_" + g.GrantNo,
			UserId:        g.UserId,
			BoxId:         g.BoxId,
			CampaignId:    participation.CampaignId,
			Status:        entitlementAvailable,
			LockedBy:      "",
			GrantedAt:     time.Now(),
		}
		if err := svcCtx.EntitlementModel.Insert(ctx, svcCtx.DB, row); err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
			return err
		}
	default:
		return fmt.Errorf("未知奖品类型: %d", g.PrizeKind)
	}
	logx.WithContext(ctx).Infof("grantOnePrize ok, grantNo: %s, kind: %d, userId: %d", g.GrantNo, g.PrizeKind, g.UserId)
	return nil
}

// insertParticipationAndGrants 事务内写入参与幂等行 + PENDING 流水（grant_no 确定性派生）
func insertParticipationAndGrants(tx *gorm.DB, idemKey string, campaignId, userId, kind int64,
	prizes []prizeCfg, resultJSON string) (int64, error) {
	now := time.Now()
	participation := &gamemodelnc.CampaignParticipations{
		IdemKey:    idemKey,
		CampaignId: campaignId,
		UserId:     userId,
		Kind:       kind,
		ResultJson: resultJSON,
		CreatedAt:  now,
	}
	if err := tx.Create(participation).Error; err != nil {
		return 0, err
	}
	for i, p := range prizes {
		grant := &gamemodelnc.CampaignPrizeGrants{
			GrantNo:          fmt.Sprintf("pg_%s#%d", idemKey, i),
			ParticipationId:  participation.Id,
			IdemKey:          idemKey,
			UserId:           userId,
			PrizeKind:        p.Kind,
			AmountFen:        p.CashFen,
			PointsAmount:     p.Points,
			CouponTemplateNo: p.CouponTemplateNo,
			BoxId:            p.BoxId,
			Status:           grantPending,
			CreatedAt:        now,
		}
		if err := tx.Create(grant).Error; err != nil {
			return 0, err
		}
	}
	return participation.Id, nil
}

// resultJSON 参与结果序列化
func marshalResult(r *participateResult) (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// unmarshalResult 重放解析（损坏兜底空结果）
func unmarshalResult(s string) *participateResult {
	r := &participateResult{}
	if err := json.Unmarshal([]byte(s), r); err != nil {
		return r
	}
	return r
}
