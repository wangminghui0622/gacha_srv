package groupbuy

import (
	"context"

	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

// purchaseMode 拼团抽购买策略：实现 saga.PurchaseMode[*types.GroupBuyBuyReq]。
type purchaseMode struct {
	log logx.Logger
}

var _ saga.PurchaseMode[*types.GroupBuyBuyReq] = purchaseMode{}

func (m purchaseMode) Name() string                               { return "groupbuy" }
func (m purchaseMode) Scene() string                              { return "GROUP_BUY" }
func (m purchaseMode) ConfigID(req *types.GroupBuyBuyReq) int64   { return req.ConfigId }
func (m purchaseMode) RequireRequestID() bool                     { return false }
func (m purchaseMode) UserCouponID(req *types.GroupBuyBuyReq) int64 { return 0 }
func (m purchaseMode) FailMsg() string                            { return "拼团抽请求提交失败，请稍后重试" }

func (m purchaseMode) LoadUnitFen(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GroupBuyBuyReq, userID int64) (int64, error) {
	detail, err := svcCtx.PlayRpc.GetGroupBuy(ctx, &playClient.GetGroupBuyReq{
		ConfigId: req.ConfigId,
		UserId:   userID,
	})
	if err != nil {
		m.log.Errorf("GroupBuyBuy GetGroupBuy failed, configId: %d, err: %v", req.ConfigId, err)
		return 0, errorx.FromError(err)
	}
	unitFen, err := utils.YuanToFen(detail.Price)
	if err != nil || unitFen <= 0 {
		return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "拼团抽价格无效")
	}
	return unitFen, nil
}

func (m purchaseMode) AmountFen(unitFen int64, req *types.GroupBuyBuyReq) int64 {
	return unitFen * req.Count
}

func (m purchaseMode) Prepare(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GroupBuyBuyReq, userID int64) error {
	return nil
}

func (m purchaseMode) Idempotent(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GroupBuyBuyReq, gid string, userID int64) (bool, error) {
	rec, err := svcCtx.PlayRpc.ListGroupBuyRecords(ctx, &playClient.ListGroupBuyRecordsReq{
		RequestId: gid,
		UserId:    userID,
	})
	if err != nil {
		return false, err
	}
	return rec != nil && len(rec.Items) > 0, nil
}

func (m purchaseMode) BuildBranches(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GroupBuyBuyReq,
	gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
	c := svcCtx.Config
	return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.ConfigId, quote, saga.PlayBranchSet{
		// 01 锁报价+下注追加+余位-count；补偿 ReleaseGroupBuy 弹注+余位+count+释放报价
		LockPath:    c.PlayRpcDirect + "/play.Play/LockGroupBuy",
		ReleasePath: c.PlayRpcDirect + "/play.Play/ReleaseGroupBuy",
		LockPayload: &playClient.LockGroupBuyReq{
			RequestId:     gid,
			QuoteToken:    quote.QuoteToken,
			ConfigId:      req.ConfigId,
			UserId:        userID,
			Count:         req.Count,
			TotalPriceFen: grossFen,
		},
		// 03 落记录+余位归零时整批结算；补偿 SettleGroupBuyRevert
		SettlePath:       c.PlayRpcDirect + "/play.Play/SettleGroupBuy",
		SettleRevertPath: c.PlayRpcDirect + "/play.Play/SettleGroupBuyRevert",
		SettlePayload: &playClient.SettleGroupBuyReq{
			RequestId: gid,
			UserId:    userID,
			ConfigId:  req.ConfigId,
			Count:     req.Count,
			Price:     utils.FenToYuan(quote.PayableFen),
		},
		// 04 发结算奖：永不 Aborted、空补偿（奖品先于捕获且不追回）
		GrantPath:       c.PlayRpcDirect + "/play.Play/GrantGroupBuyPrizes",
		GrantRevertPath: c.PlayRpcDirect + "/play.Play/GrantGroupBuyPrizesRevert",
		GrantPayload: &playClient.GrantGroupBuyPrizesReq{
			RequestId: gid,
			UserId:    userID,
			ConfigId:  req.ConfigId,
		},
	}), nil
}

func (m purchaseMode) Recheck(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GroupBuyBuyReq, gid string, userID int64) error {
	resp, err := svcCtx.PlayRpc.ListGroupBuyRecords(ctx, &playClient.ListGroupBuyRecordsReq{
		RequestId: gid,
		UserId:    userID,
	})
	if err != nil {
		return err
	}
	if resp == nil || len(resp.Items) == 0 {
		return saga.ErrRecordNotFound
	}
	return nil
}
