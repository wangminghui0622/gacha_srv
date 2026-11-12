package gameimpl

import (
	"context"
	"errors"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CheckPrizesGrantedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckPrizesGrantedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckPrizesGrantedLogic {
	return &CheckPrizesGrantedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CheckPrizesGranted SAGA 04 补偿（CancelPayment）退款前置检查：
// 按 gid 查 06 发奖落库标记（prize_grant_marks uk request_id）。五条发奖链
// （gashapon/dig/fullcar/groupbuy/combochain）的 06 在发奖循环成功后写入标记；
// 标记存在 ⇒ 奖品已发放（现金/积分已入账、券已落库），payment 补偿不得退钱
// （资产已发放不追回，钱保持扣除，冻结保持可恢复）。
// 幂等：纯读查询，重复调用安全；RPC 故障返回 error 供 DTM 重试补偿。
func (l *CheckPrizesGrantedLogic) CheckPrizesGranted(in *play.CheckPrizesGrantedReq) (*play.CheckPrizesGrantedResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}
	_, err := l.svcCtx.PrizeGrantMarkModel.FindOneByRequestId(l.ctx, in.RequestId)
	if err != nil {
		if errors.Is(err, gamemodelnc.ErrNotFound) {
			return &play.CheckPrizesGrantedResp{Granted: false}, nil
		}
		return nil, err
	}
	return &play.CheckPrizesGrantedResp{Granted: true}, nil
}

// ══════════════ 玩法发奖公共常量 ═══════════════
// 发奖类型与 wallet_kind 对齐（旧仓 campaignOps 定义，玩法域共用拆出）。

// 奖品类型（prizes_json 元素 kind / grant 分发键）
const (
	prizeCash   int64 = 1 // 现金（CreditWallet kind=1）
	prizePoints int64 = 2 // 积分（CreditWallet kind=4）
	prizeCoupon int64 = 3 // 券（"uc_"+grantNo 确定性单号）
)

// couponNameOf 经营销服务查券名（展示用；NotFound/系统错误静默降级返回空——旧仓同款容错）
func couponNameOf(ctx context.Context, svcCtx *svc.ServiceContext, templateNo string) string {
	if templateNo == "" {
		return ""
	}
	tpl, err := svcCtx.MarketingRpc.GetCouponTemplateByNo(ctx, &marketingClient.GetCouponTemplateByNoReq{TemplateNo: templateNo})
	if err != nil {
		return ""
	}
	return tpl.GetName()
}
