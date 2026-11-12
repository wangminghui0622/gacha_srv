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

type FullCarBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入）
	requestId string
}

func NewFullCarBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FullCarBuyLogic {
	return &FullCarBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *FullCarBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// FullCarBuy 满车购座：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支，与 DigBuy/GashaponBuy 同构）。
//
//	[01 play.LockFullCar 锁报价+占座+余票-1 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleFullCar 抽 regular 奖+落记录（最后一票触发整车结算）
//	 → 04 play.GrantFullCarPrizes 发整车奖（永不 Aborted，空补偿）
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseFullCar 释座+余票+1+释放报价 → CancelPayment →
//	      SettleFullCarRevert 撤销记录/整车奖/库存 → 空 → 空]
//	同步预检：base.EvaluatePolicy（scene=FULL_CAR）+ base.RiskEvaluate，拒绝立即返回 4001/4002。
func (l *FullCarBuyLogic) FullCarBuy(req *types.FullCarBuyReq) (*types.FullCarBuyResp, error) {
	flow := &saga.BuyFlow{
		Ctx:       l.ctx,
		Log:       l.Logger,
		Svc:       l.svcCtx,
		RequestID: l.requestId,
		Scene:     "FULL_CAR",
		ConfigID:  req.ConfigId,
		FailMsg:   "满车请求提交失败，请稍后重试",
		LoadUnitFen: func(ctx context.Context, userID int64) (int64, error) {
			detail, err := l.svcCtx.PlayRpc.GetFullCar(ctx, &playClient.GetFullCarReq{ConfigId: req.ConfigId})
			if err != nil {
				l.Errorf("FullCarBuy GetFullCar failed, configId: %d, err: %v", req.ConfigId, err)
				return 0, errorx.FromError(err)
			}
			unitFen, err := utils.YuanToFen(detail.Price)
			if err != nil || unitFen <= 0 {
				return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "满车价格无效")
			}
			return unitFen, nil
		},
		Idempotent: func(ctx context.Context, gid string, userID int64) (bool, error) {
			rec, err := l.svcCtx.PlayRpc.ListFullCarRecords(ctx, &playClient.ListFullCarRecordsReq{
				RequestId: gid,
				UserId:    userID,
			})
			if err != nil {
				return false, err
			}
			return rec != nil && len(rec.Items) > 0, nil
		},
		BuildBranches: func(gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
			c := l.svcCtx.Config
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
		},
		Recheck: func(ctx context.Context, gid string, userID int64) error {
			resp, err := l.svcCtx.PlayRpc.ListFullCarRecords(ctx, &playClient.ListFullCarRecordsReq{
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
		},
	}

	gid, userID, alreadyDone, err := flow.Run()
	if err != nil {
		return nil, err
	}
	status := "pending"
	if alreadyDone {
		status = "success"
	}
	l.Infof("full car buy submitted, gid: %s, configId: %d, userId: %d, seatIndex: %d, status: %s",
		gid, req.ConfigId, userID, req.SeatIndex, status)
	return &types.FullCarBuyResp{RequestId: gid, Status: status}, nil
}
