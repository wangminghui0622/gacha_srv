// Package playkit 玩法域公共能力。
//
// 目标：把 6 个玩法里重复的「逐奖幂等发放 + grant_no 派生 + 发奖落库标记」
// 收敛到一处，玩法只负责把自己的 award 结构映射成 Prize。
package playkit

import (
	"context"
	"errors"
	"fmt"

	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/money/moneyClient"
	"gacha_srv/app/play/internal/svc"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"

	"gorm.io/gorm"
)

// 奖品类型（与各玩法 prizes_json / awards_json 的 kind 对齐）
const (
	KindCash   int64 = 1 // 现金
	KindPoints int64 = 2 // 积分
	KindCoupon int64 = 3 // 券
	KindGoods  int64 = 5 // 实物 SKU（catalog.skus.id）
)

// 钱包类型（money.CreditWallet kind）
const (
	WalletCash         int32 = 1
	WalletLoyaltyPoint int32 = 4
)

// Prize 统一奖品形状：各玩法的 award 结构映射到它，发奖逻辑只写一份。
type Prize struct {
	UserID           int64
	Kind             int64
	CashFen          int64
	Points           int64
	CouponTemplateNo string
	SkuId            int64  // KindGoods：catalog SKU ID
	ItemName         string // KindGoods：奖品名称快照
	PoolId           int64  // KindGoods：玩法池/配置 ID（落资产行）
}

// GrantAll 逐奖幂等发放：grant_no = prefix_requestID#idx（确定性派生）。
//
// 失败立即返回；DTM 重试时同一 grant_no 命中钱包 requestId / 券 uk，幂等收敛。
// 空列表直接成功（无奖也是合法结果）。
func GrantAll(ctx context.Context, svcCtx *svc.ServiceContext, requestID, prefix string, prizes []Prize) error {
	for i, p := range prizes {
		grantNo := fmt.Sprintf("%s_%s#%d", prefix, requestID, i)
		if err := GrantOne(ctx, svcCtx, p.UserID, grantNo, p); err != nil {
			return fmt.Errorf("grant prize %d: %w", i, err)
		}
	}
	return nil
}

// GrantOne 单奖幂等发放：
//   - 现金 → money.CreditWallet（kind=1）
//   - 积分 → money.CreditWallet（kind=4）
//   - 券   → marketing.GrantPlayCoupon（coupon_no="uc_"+grantNo 幂等）
//   - 实物 → money.GrantPlayAsset（asset_no="pa_"+grantNo 幂等）
//   - 其它 → no-op（无奖普通条目，如拼团普通奖）
func GrantOne(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, grantNo string, p Prize) error {
	switch p.Kind {
	case KindCash:
		_, err := svcCtx.MoneyRpc.CreditWallet(ctx, &moneyClient.CreditWalletReq{
			UserId:    userID,
			Kind:      WalletCash,
			Amount:    utils.FenToYuan(p.CashFen),
			RequestId: grantNo,
		})
		return err
	case KindPoints:
		_, err := svcCtx.MoneyRpc.CreditWallet(ctx, &moneyClient.CreditWalletReq{
			UserId:    userID,
			Kind:      WalletLoyaltyPoint,
			Amount:    utils.FenToYuan(p.Points),
			RequestId: grantNo,
		})
		return err
	case KindCoupon:
		_, err := svcCtx.MarketingRpc.GrantPlayCoupon(ctx, &marketingClient.GrantPlayCouponReq{
			RequestId:  grantNo,
			UserId:     userID,
			TemplateNo: p.CouponTemplateNo,
		})
		return err
	case KindGoods:
		_, err := svcCtx.MoneyRpc.GrantPlayAsset(ctx, &moneyClient.GrantPlayAssetReq{
			GrantNo:  grantNo,
			UserId:   userID,
			PoolId:   p.PoolId,
			SkuId:    p.SkuId,
			ItemName: p.ItemName,
		})
		return err
	default:
		return nil
	}
}

// MarkGranted 按 gid 写发奖落库标记（uk request_id；撞键=幂等成功）。
// 04 补偿（CancelPayment）退款前经 CheckPrizesGranted 查此标记，已发奖则不退钱。
func MarkGranted(ctx context.Context, svcCtx *svc.ServiceContext, requestID string, userID int64) error {
	err := svcCtx.PrizeGrantMarkModel.Insert(ctx, nil, &gamemodelnc.PrizeGrantMarks{
		RequestId: requestID,
		UserId:    userID,
	})
	if err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return err
	}
	return nil
}
