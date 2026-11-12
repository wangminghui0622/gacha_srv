package gashapon

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

// purchaseMode 扭蛋购买策略：实现 saga.PurchaseMode[*types.GashaponBuyReq]。
// 网关 RunBuy 负责通用编排，这里只保留扭蛋自己的单价/幂等/分支。
type purchaseMode struct {
	log logx.Logger
}

var _ saga.PurchaseMode[*types.GashaponBuyReq] = purchaseMode{}

func (m purchaseMode) Name() string                              { return "gashapon" }
func (m purchaseMode) Scene() string                             { return "GASHAPON" }
func (m purchaseMode) ConfigID(req *types.GashaponBuyReq) int64  { return req.ConfigId }
func (m purchaseMode) RequireRequestID() bool                    { return false }
func (m purchaseMode) UserCouponID(req *types.GashaponBuyReq) int64 { return 0 }
func (m purchaseMode) FailMsg() string                           { return "扭蛋请求提交失败，请稍后重试" }

func (m purchaseMode) LoadUnitFen(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GashaponBuyReq, userID int64) (int64, error) {
	detail, err := svcCtx.PlayRpc.GetGashapon(ctx, &playClient.GetGashaponReq{ConfigId: req.ConfigId})
	if err != nil {
		m.log.Errorf("GashaponBuy GetGashapon failed, configId: %d, err: %v", req.ConfigId, err)
		return 0, errorx.FromError(err)
	}
	unitFen, err := utils.YuanToFen(detail.Price)
	if err != nil || unitFen <= 0 {
		return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "扭蛋价格无效")
	}
	return unitFen, nil
}

func (m purchaseMode) AmountFen(unitFen int64, req *types.GashaponBuyReq) int64 {
	return unitFen * req.Count
}

func (m purchaseMode) Prepare(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GashaponBuyReq, userID int64) error {
	return nil
}

func (m purchaseMode) Idempotent(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GashaponBuyReq, gid string, userID int64) (bool, error) {
	rec, err := svcCtx.PlayRpc.ListGashaponRecords(ctx, &playClient.ListGashaponRecordsReq{
		RequestId: gid,
		UserId:    userID,
	})
	if err != nil {
		return false, err
	}
	return rec != nil && len(rec.Items) > 0, nil
}

func (m purchaseMode) BuildBranches(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GashaponBuyReq,
	gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
	c := svcCtx.Config
	return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.ConfigId, quote, saga.PlayBranchSet{
		// 01 锁报价+参与记录+余票-count；补偿 ReleaseGashapon 弹记录+余票+count+释放报价
		LockPath:    c.PlayRpcDirect + "/play.Play/LockGashapon",
		ReleasePath: c.PlayRpcDirect + "/play.Play/ReleaseGashapon",
		LockPayload: &playClient.LockGashaponReq{
			RequestId:     gid,
			QuoteToken:    quote.QuoteToken,
			ConfigId:      req.ConfigId,
			UserId:        userID,
			Count:         req.Count,
			TotalPriceFen: grossFen,
		},
		// 03 抽 count 个 regular 奖+特别奖判定+落记录；补偿 SettleGashaponRevert
		SettlePath:       c.PlayRpcDirect + "/play.Play/SettleGashapon",
		SettleRevertPath: c.PlayRpcDirect + "/play.Play/SettleGashaponRevert",
		SettlePayload: &playClient.SettleGashaponReq{
			RequestId: gid,
			UserId:    userID,
			ConfigId:  req.ConfigId,
			Count:     req.Count,
			Price:     utils.FenToYuan(quote.PayableFen),
		},
		// 04 发特别奖：永不 Aborted、空补偿（奖品先于捕获且不追回）
		GrantPath:       c.PlayRpcDirect + "/play.Play/GrantGashaponPrizes",
		GrantRevertPath: c.PlayRpcDirect + "/play.Play/GrantGashaponPrizesRevert",
		GrantPayload: &playClient.GrantGashaponPrizesReq{
			RequestId: gid,
			UserId:    userID,
			ConfigId:  req.ConfigId,
		},
	}), nil
}

func (m purchaseMode) Recheck(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GashaponBuyReq, gid string, userID int64) error {
	resp, err := svcCtx.PlayRpc.ListGashaponRecords(ctx, &playClient.ListGashaponRecordsReq{
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
