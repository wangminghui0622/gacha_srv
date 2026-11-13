package gameimpl

import (
	"context"
	"encoding/json"
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

// digGrantPrizesOfRecord 记录 prizes_json → 发放队列（kind=0 空格已在落库时过滤）
func digGrantPrizesOfRecord(prizesJSON string) ([]digCellPrize, error) {
	if prizesJSON == "" {
		return nil, nil
	}
	var prizes []digCellPrize
	if err := json.Unmarshal([]byte(prizesJSON), &prizes); err != nil {
		return nil, err
	}
	return prizes, nil
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
	items := make([]playkit.Prize, 0, len(prizes))
	for _, p := range prizes {
		items = append(items, playkit.Prize{
			UserID:           in.UserId,
			Kind:             p.Kind,
			CashFen:          p.CashFen,
			Points:           p.Points,
			CouponTemplateNo: p.CouponTemplateNo,
			SkuId:            p.SkuId,
			ItemName:         p.ItemName,
			PoolId:           rec.ConfigId,
		})
	}

	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "dgp", items); err != nil {

		l.Errorf("GrantDigPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if len(items) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantDigPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantDigPrizes success, requestId: %s, prizes: %d", in.RequestId, len(items))
	return &play.GrantDigPrizesResp{Status: "GRANTED"}, nil
}

func NewGrantDigPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantDigPrizesRevertLogic {
	return &GrantDigPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantDigPrizesRevert SAGA 06 补偿：空操作（D4 决策——奖品先于捕获且不追回，
// 补偿注册仅为防御；即使调用也幂等返回）
func (l *GrantDigPrizesRevertLogic) GrantDigPrizesRevert(in *play.GrantDigPrizesReq) (*play.GrantDigPrizesResp, error) {
	l.Infof("GrantDigPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantDigPrizesResp{Status: "GRANTED"}, nil
}
