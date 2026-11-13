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
	prizes := make([]playkit.Prize, 0, len(awards))
	for _, a := range awards {
		prizes = append(prizes, playkit.Prize{
			UserID:           a.UserID,
			Kind:             a.PrizeKind,
			CashFen:          a.CashFen,
			Points:           a.PointsAmount,
			CouponTemplateNo: a.CouponTemplateNo,
		})
	}

	// 普通实物奖：regular 抽取结果落 records.entries_json，只发 sku_id>0 的条目
	var rec gamemodelnc.GashaponRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).First(&rec).Error; err != nil {
		l.Errorf("GrantGashaponPrizes record not found, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if rec.Status == gashaponRecordNormal {
		for _, o := range gashaponParseOutcomes(rec.EntriesJson) {
			if o.SkuId > 0 {
				prizes = append(prizes, playkit.Prize{
					UserID:   in.UserId,
					Kind:     prizeGoods,
					SkuId:    o.SkuId,
					ItemName: o.Name,
					PoolId:   in.ConfigId,
				})
			}
		}
	}

	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "gcp", prizes); err != nil {

		l.Errorf("GrantGashaponPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if len(prizes) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantGashaponPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantGashaponPrizes success, requestId: %s, awards: %d", in.RequestId, len(awards))
	return &play.GrantGashaponPrizesResp{Status: "GRANTED"}, nil
}

func NewGrantGashaponPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGashaponPrizesRevertLogic {
	return &GrantGashaponPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGashaponPrizesRevert SAGA 06 补偿：空操作（特别奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantGashaponPrizesRevertLogic) GrantGashaponPrizesRevert(in *play.GrantGashaponPrizesReq) (*play.GrantGashaponPrizesResp, error) {
	l.Infof("GrantGashaponPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantGashaponPrizesResp{Status: "GRANTED"}, nil
}
