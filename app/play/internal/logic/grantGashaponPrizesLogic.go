package logic

import (
	"context"
	"errors"
	"fmt"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type GrantGashaponPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantGashaponPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGashaponPrizesLogic {
	return &GrantGashaponPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGashaponPrizes SAGA 06 正向：发放本 gid 触发的特别奖（随 05 落 awards_json，
// 按 requestId 精确定位；无特别奖时为空集、直接成功）。
// 逐奖幂等发放（grant_no = "gcp_"+gid+"#"+idx，确定性派生）：
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//
// regular 奖（扭蛋抽中条目）只落记录展示、不在此发放（条目无 prize_kind，
// 形状同 draw_pool_entries 的资产化由 draw 域负责；扭蛋条目仅展示+特别奖实发）。
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 dig GrantDigPrizes 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantGashaponPrizesLogic) GrantGashaponPrizes(in *play.GrantGashaponPrizesReq) (*play.GrantGashaponPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 特别奖由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var st gamemodelnc.GashaponState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantGashaponPrizes state not found, configId: %d", in.ConfigId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=扭蛋状态不存在", errorx.CodeGameConfigInvalid))
		}
		l.Errorf("GrantGashaponPrizes query state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
	if err != nil {
		l.Errorf("GrantGashaponPrizes parse state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	awards := gashaponAwardsOfRequest(state.Awards, in.RequestId)
	for i, a := range awards {
		// grantNo 请求作用域幂等键（gcp_{gid}#{idx}，确定性派生）：DTM 重试/补偿重放
		// 同一奖项命中同一键，CreditWallet requestId 幂等 + 券 uk 撞键 = 已发过读回成功
		grantNo := fmt.Sprintf("gcp_%s#%d", in.RequestId, i)
		if err := grantOneGashaponPrize(l.ctx, l.svcCtx, a.UserID, grantNo, a); err != nil {
			// 永不 Aborted：失败 Internal → DTM 重试，grant_no 幂等收敛
			l.Errorf("GrantGashaponPrizes grant failed, requestId: %s, idx: %d, err: %v", in.RequestId, i, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	// 高1 修复：发奖落库标记（04 补偿退款前置检查依据）；Internal 失败 → DTM 重试 06 幂等收敛
	if len(awards) > 0 {
		if err := insertPrizeGrantMark(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantGashaponPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantGashaponPrizes success, requestId: %s, awards: %d", in.RequestId, len(awards))
	return &play.GrantGashaponPrizesResp{Status: "GRANTED"}, nil
}
