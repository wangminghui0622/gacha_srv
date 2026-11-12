package fullcar

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

// purchaseMode 满车购买策略：实现 saga.PurchaseMode[*types.FullCarBuyReq]。
type purchaseMode struct {
	log logx.Logger
}

var _ saga.PurchaseMode[*types.FullCarBuyReq] = purchaseMode{}

func (m purchaseMode) Name() string                               { return "fullcar" }
func (m purchaseMode) Scene() string                              { return "FULL_CAR" }
func (m purchaseMode) ConfigID(req *types.FullCarBuyReq) int64    { return req.ConfigId }
func (m purchaseMode) RequireRequestID() bool                     { return false }
func (m purchaseMode) UserCouponID(req *types.FullCarBuyReq) int64 { return 0 }
func (m purchaseMode) FailMsg() string                            { return "满车请求提交失败，请稍后重试" }

func (m purchaseMode) LoadUnitFen(ctx context.Context, svcCtx *svc.ServiceContext, req *types.FullCarBuyReq, userID int64) (int64, error) {
	detail, err := svcCtx.PlayRpc.GetFullCar(ctx, &playClient.GetFullCarReq{ConfigId: req.ConfigId})
	if err != nil {
		m.log.Errorf("FullCarBuy GetFullCar failed, configId: %d, err: %v", req.ConfigId, err)
		return 0, errorx.FromError(err)
	}
	unitFen, err := utils.YuanToFen(detail.Price)
	if err != nil || unitFen <= 0 {
		return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "满车价格无效")
	}
	return unitFen, nil
}

func (m purchaseMode) AmountFen(unitFen int64, req *types.FullCarBuyReq) int64 { return unitFen }

func (m purchaseMode) Prepare(ctx context.Context, svcCtx *svc.ServiceContext, req *types.FullCarBuyReq, userID int64) error {
	return nil
}

func (m purchaseMode) Idempotent(ctx context.Context, svcCtx *svc.ServiceContext, req *types.FullCarBuyReq, gid string, userID int64) (bool, error) {
	rec, err := svcCtx.PlayRpc.ListFullCarRecords(ctx, &playClient.ListFullCarRecordsReq{
		RequestId: gid,
		UserId:    userID,
	})
	if err != nil {
		return false, err
	}
	return rec != nil && len(rec.Items) > 0, nil
}

func (m purchaseMode) BuildBranches(ctx context.Context, svcCtx *svc.ServiceContext, req *types.FullCarBuyReq,
	gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
	c := svcCtx.Config
	return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.ConfigId, quote, saga.PlayBranchSet{
		// 01 锁报价+占座+余票-1；补偿 ReleaseFullCar 释座+余票+1+释放报价
		LockPath:    c.PlayRpcDirect + "/play.Play/LockFullCar",
		ReleasePath: c.PlayRpcDirect + "/play.Play/ReleaseFullCar",
		LockPayload: &playClient.LockFullCarReq{
			RequestId:     gid,
			QuoteToken:    quote.QuoteToken,
			ConfigId:      req.ConfigId,
			UserId:        userID,
			SeatIndex:     req.SeatIndex,
			TotalPriceFen: unitFen,
		},
		// 03 抽 regular 奖+落记录；补偿 SettleFullCarRevert 撤销记录/整车奖/库存
		SettlePath:       c.PlayRpcDirect + "/play.Play/SettleFullCar",
		SettleRevertPath: c.PlayRpcDirect + "/play.Play/SettleFullCarRevert",
		SettlePayload: &playClient.SettleFullCarReq{
			RequestId: gid,
			UserId:    userID,
			ConfigId:  req.ConfigId,
			SeatIndex: req.SeatIndex,
			Price:     utils.FenToYuan(quote.PayableFen),
		},
		// 04 发整车奖：永不 Aborted、空补偿（奖品先于捕获且不追回）
		GrantPath:       c.PlayRpcDirect + "/play.Play/GrantFullCarPrizes",
		GrantRevertPath: c.PlayRpcDirect + "/play.Play/GrantFullCarPrizesRevert",
		GrantPayload: &playClient.GrantFullCarPrizesReq{
			RequestId: gid,
			UserId:    userID,
			ConfigId:  req.ConfigId,
		},
	}), nil
}

func (m purchaseMode) Recheck(ctx context.Context, svcCtx *svc.ServiceContext, req *types.FullCarBuyReq, gid string, userID int64) error {
	resp, err := svcCtx.PlayRpc.ListFullCarRecords(ctx, &playClient.ListFullCarRecordsReq{
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
