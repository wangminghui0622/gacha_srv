package campaign

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/logic/draw"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

// CampaignBoxOpenLogic 活动宝箱开箱：entitlement 资格全抵 → 零付开箱（复用抽奖 7 分支 SAGA）
type CampaignBoxOpenLogic struct {
	logx.Logger
	ctx       context.Context
	svcCtx    *svc.ServiceContext
	requestId string // Idempotency-Key 头（可选，同 BoxBuy）
}

func NewCampaignBoxOpenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CampaignBoxOpenLogic {
	return &CampaignBoxOpenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *CampaignBoxOpenLogic) SetRequestId(id string) {
	l.requestId = id
}

// OpenCampaignBox 开箱流程（M11c）：
//
//	① gid = Idempotency-Key 或 uuid；同 key 重放快路径必须先于资格校验——
//	   开箱成功后资格已 USED，先校验会 5014，永远到不了 DTM 重复 gid 兜底（M11b 教训）
//	② GetCampaignBoxEntitlement 校验资格（归属/AVAILABLE，5013/5014，marketing 域裁决）
//	③ BoxDetail 取箱价（服务端定价）+ 售罄预检（快速失败 UX；SettleDraw 最终裁决）
//	④ QuoteCheckout(gross=箱价, entitlementNo) → 资格全抵 payable=0（marketing 域裁决）
//	⑤ SubmitDrawSaga（同步预检资格+风控后 5 分支：01 LockQuote 锁资格 AVAILABLE→LOCKED；05 CommitQuote 核销 USED）
//	⑥ 返回 {requestId}，前端轮询 /draw/records?requestId=
func (l *CampaignBoxOpenLogic) OpenCampaignBox(req *types.OpenCampaignBoxReq) (resp *types.OpenCampaignBoxResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}

	// ① gid = Idempotency-Key 头（重试场景复用）或服务端 uuid
	gid := uuid.New().String()
	if l.requestId != "" {
		if len(l.requestId) > 64 {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "幂等键过长（最长 64）")
		}
		gid = l.requestId
		// 幂等重放快路径：同 key 已有开奖记录 → 直接 success
		if _, qe := l.svcCtx.DrawRpc.GetRecord(l.ctx, &drawClient.GetRecordReq{
			RequestId: gid,
			UserId:    userId,
		}); qe == nil {
			l.Infof("retry with same idempotency key, returning existing record, gid: %s", gid)
			return &types.OpenCampaignBoxResp{RequestId: gid, Status: "success"}, nil
		}
	}

	// ② 资格校验（marketing 域裁决：存在/归属/AVAILABLE → 5013/5014）
	ent, err := l.svcCtx.MarketingRpc.GetCampaignBoxEntitlement(l.ctx, &marketingClient.GetCampaignBoxEntitlementReq{
		UserId:        userId,
		EntitlementNo: req.EntitlementNo,
	})
	if err != nil {
		l.Errorf("OpenCampaignBox GetCampaignBoxEntitlement failed, entitlementNo: %s, err: %v", req.EntitlementNo, err)
		return nil, errorx.FromError(err)
	}

	// ③ 箱价取服务端（Etcd 调 draw 服务），不信客户端
	detail, err := l.svcCtx.DrawRpc.BoxDetail(l.ctx, &drawClient.BoxDetailReq{BoxId: ent.BoxId})
	if err != nil {
		l.Errorf("OpenCampaignBox BoxDetail failed, boxId: %d, err: %v", ent.BoxId, err)
		return nil, errorx.FromError(err)
	}
	if detail.SoldOut {
		return nil, errorx.NewCodeError(errorx.CodeBoxSoldOut, "宝箱已售罄")
	}
	amountFen, err := utils.YuanToFen(detail.Price)
	if err != nil || amountFen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAmountInvalid, "宝箱价格无效")
	}

	// ④ 报价：资格全抵（marketing 域校验归属/状态/箱型，couponDiscountFen=gross → payable=0）
	quote, err := l.svcCtx.MarketingRpc.QuoteCheckout(l.ctx, &marketingClient.QuoteCheckoutReq{
		RequestId:     gid,
		UserId:        userId,
		BoxId:         ent.BoxId,
		EntitlementNo: req.EntitlementNo,
		GrossFen:      amountFen,
	})
	if err != nil {
		l.Errorf("OpenCampaignBox QuoteCheckout failed, gid: %s, err: %v", gid, err)
		return nil, errorx.FromError(err)
	}

	// ⑤ 组装并提交 7 分支 SAGA（共享构建器；SettleDraw price=FenToYuan(payable=0)="0.00"）
	alreadyDone, err := draw.SubmitDrawSaga(l.ctx, l.svcCtx, l.Logger, gid, l.requestId, userId, ent.BoxId, amountFen, quote)
	if err != nil {
		return nil, err
	}
	if alreadyDone {
		return &types.OpenCampaignBoxResp{RequestId: gid, Status: "success"}, nil
	}
	return &types.OpenCampaignBoxResp{RequestId: gid, Status: "pending"}, nil
}
