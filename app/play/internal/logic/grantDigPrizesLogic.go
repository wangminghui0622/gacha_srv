package logic

import (
	"context"
	"errors"
	"fmt"

	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/money/moneyClient"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type GrantDigPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantDigPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantDigPrizesLogic {
	return &GrantDigPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantDigPrizes SAGA 06 正向：按记录 prizes_json 逐奖幂等发放。
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//     grant_no = "dgp_"+requestId+"#"+idx（确定性派生，重试/重放只补发不重复）。
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 draw 域 GrantAsset 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantDigPrizesLogic) GrantDigPrizes(in *play.GrantDigPrizesReq) (*play.GrantDigPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 记录由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var rec gamemodelnc.DigBoxRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantDigPrizes record not found, requestId: %s", in.RequestId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=挖宝记录不存在", errorx.CodeDigRecordNotFound))
		}
		l.Errorf("GrantDigPrizes query record failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	prizes, err := digGrantPrizesOfRecord(rec.PrizesJson)
	if err != nil {
		l.Errorf("GrantDigPrizes parse prizes failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	for i, p := range prizes {
		grantNo := fmt.Sprintf("dgp_%s#%d", in.RequestId, i)
		if err := grantOneDigPrize(l.ctx, l.svcCtx, in.UserId, grantNo, p); err != nil {
			// 永不 Aborted：失败 Internal → DTM 重试，grant_no 幂等收敛
			l.Errorf("GrantDigPrizes grant failed, requestId: %s, idx: %d, err: %v", in.RequestId, i, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	// 高1 修复：发奖落库标记（04 补偿退款前置检查依据）；Internal 失败 → DTM 重试 06 幂等收敛
	if len(prizes) > 0 {
		if err := insertPrizeGrantMark(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantDigPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantDigPrizes success, requestId: %s, prizes: %d", in.RequestId, len(prizes))
	return &play.GrantDigPrizesResp{Status: "GRANTED"}, nil
}

// grantOneDigPrize 单奖幂等发放（与 campaign grantOnePrize 同款模式，grant_no 全 uk 幂等）
func grantOneDigPrize(ctx context.Context, svcCtx *svc.ServiceContext, userId int64, grantNo string, p digCellPrize) error {
	switch p.Kind {
	case prizeCash:
		if _, err := svcCtx.MoneyRpc.CreditWallet(ctx, &moneyClient.CreditWalletReq{
			UserId:    userId,
			Kind:      walletCash,
			Amount:    utils.FenToYuan(p.CashFen),
			RequestId: grantNo,
		}); err != nil {
			return err
		}
	case prizePoints:
		if _, err := svcCtx.MoneyRpc.CreditWallet(ctx, &moneyClient.CreditWalletReq{
			UserId:    userId,
			Kind:      walletLoyaltyPoint,
			Amount:    utils.FenToYuan(p.Points),
			RequestId: grantNo,
		}); err != nil {
			return err
		}
	case prizeCoupon:
		// 券经营销 RPC 发放（coupon_no="uc_"+grantNo 撞键幂等；旧同服务直插 user_coupons 表）
		if _, err := svcCtx.MarketingRpc.GrantPlayCoupon(ctx, &marketingClient.GrantPlayCouponReq{
			RequestId:  grantNo,
			UserId:     userId,
			TemplateNo: p.CouponTemplateNo,
		}); err != nil {
			return err
		}
	default:
		// kind=0（无奖普通条目，如拼团普通奖）no-op：不发放、不报错
		return nil
	}
	return nil
}
