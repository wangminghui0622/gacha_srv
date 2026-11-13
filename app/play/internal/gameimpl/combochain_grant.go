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

func NewGrantComboChainPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantComboChainPrizesLogic {
	return &GrantComboChainPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantComboChainPrizes SAGA 06 正向：发放本 gid 触发的结算奖（随 05 落 awards_json，
// 按 requestId 精确定位；非结算票时为空集、直接成功）。
// 逐奖幂等发放（grant_no = "ccp_"+gid+"#"+idx，确定性派生）：
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 dig GrantDigPrizes 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantComboChainPrizesLogic) GrantComboChainPrizes(in *play.GrantComboChainPrizesReq) (*play.GrantComboChainPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 结算奖由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantComboChainPrizes state not found, configId: %d", in.ConfigId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=连击链状态不存在", errorx.CodeGameConfigInvalid))
		}
		l.Errorf("GrantComboChainPrizes query state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
	if err != nil {
		l.Errorf("GrantComboChainPrizes parse state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	awards := comboChainAwardsOfRequest(state.Awards, in.RequestId)
	prizes := make([]playkit.Prize, 0, len(awards))
	for _, a := range awards {
		prizes = append(prizes, playkit.Prize{
			UserID:           a.UserID,
			Kind:             a.PrizeKind,
			CashFen:          a.CashFen,
			Points:           a.PointsAmount,
			CouponTemplateNo: a.CouponTemplateNo,
			SkuId:            a.SkuId,
			ItemName:         a.ItemName,
			PoolId:           in.ConfigId,
		})
	}

	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "ccp", prizes); err != nil {

		l.Errorf("GrantComboChainPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if len(prizes) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantComboChainPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantComboChainPrizes success, requestId: %s, awards: %d", in.RequestId, len(awards))
	return &play.GrantComboChainPrizesResp{Status: "GRANTED"}, nil
}

func NewGrantComboChainPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantComboChainPrizesRevertLogic {
	return &GrantComboChainPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantComboChainPrizesRevert SAGA 06 补偿：空操作（结算奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantComboChainPrizesRevertLogic) GrantComboChainPrizesRevert(in *play.GrantComboChainPrizesReq) (*play.GrantComboChainPrizesResp, error) {
	l.Infof("GrantComboChainPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantComboChainPrizesResp{Status: "GRANTED"}, nil
}
