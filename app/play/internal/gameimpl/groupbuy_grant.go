package gameimpl

import (
	"context"
	"errors"
	"fmt"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

func NewGrantGroupBuyPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGroupBuyPrizesLogic {
	return &GrantGroupBuyPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGroupBuyPrizes SAGA 06 正向：发放本 gid 触发的结算奖（随 05 落 awards_json，
// 按 requestId 精确定位；非成团注时为空集、直接成功）。
// 逐奖幂等发放（grant_no = "gbp_"+gid+"#"+idx，确定性派生）：
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 dig GrantDigPrizes 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantGroupBuyPrizesLogic) GrantGroupBuyPrizes(in *play.GrantGroupBuyPrizesReq) (*play.GrantGroupBuyPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 结算奖由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantGroupBuyPrizes state not found, configId: %d", in.ConfigId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=拼团状态不存在", errorx.CodeGameConfigInvalid))
		}
		l.Errorf("GrantGroupBuyPrizes query state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
	if err != nil {
		l.Errorf("GrantGroupBuyPrizes parse state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	awards := groupBuyAwardsOfRequest(state.Awards, in.RequestId)
	prizes := make([]playkit.Prize, 0, len(awards))
	for _, a := range awards {
		itemName := a.ItemName
		if a.PrizeKind == prizeGoods && itemName == "" {
			itemName = a.EntryName
		}
		prizes = append(prizes, playkit.Prize{
			UserID:           a.UserID,
			Kind:             a.PrizeKind,
			CashFen:          a.CashFen,
			Points:           a.PointsAmount,
			CouponTemplateNo: a.CouponTemplateNo,
			SkuId:            a.SkuId,
			ItemName:         itemName,
			PoolId:           in.ConfigId,
		})
	}

	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "gbp", prizes); err != nil {

		l.Errorf("GrantGroupBuyPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if len(prizes) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantGroupBuyPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantGroupBuyPrizes success, requestId: %s, awards: %d", in.RequestId, len(awards))
	return &play.GrantGroupBuyPrizesResp{Status: "GRANTED"}, nil
}

func NewGrantGroupBuyPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGroupBuyPrizesRevertLogic {
	return &GrantGroupBuyPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGroupBuyPrizesRevert SAGA 06 补偿：空操作（结算奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantGroupBuyPrizesRevertLogic) GrantGroupBuyPrizesRevert(in *play.GrantGroupBuyPrizesReq) (*play.GrantGroupBuyPrizesResp, error) {
	l.Infof("GrantGroupBuyPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantGroupBuyPrizesResp{Status: "GRANTED"}, nil
}
