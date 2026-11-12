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

type GashaponBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入）
	requestId string
}

func NewGashaponBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GashaponBuyLogic {
	return &GashaponBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *GashaponBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// GashaponBuy 扭蛋购买：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支，与 DigBuy/FullCarBuy 同构）。
//
//	[01 play.LockGashapon 锁报价+参与记录+余票-count
//	 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleGashapon 抽 count 个 regular 奖+分阶段 LUCKY 判定+清空时终局
//	  LAST/MVP/LUCKY+落记录 → 04 play.GrantGashaponPrizes 发特别奖（永不 Aborted，空补偿）
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseGashapon 弹记录+余票+count+释放报价 → CancelPayment →
//	      SettleGashaponRevert 撤销记录/特别奖/sent/库存 → 空 → 空]
//	同步预检：base.EvaluatePolicy（scene=GASHAPON）+ base.RiskEvaluate，拒绝立即返回 4001/4002。
//
// 安全不变式：
//  1. userId 取 JWT 上下文，不接受客户端传入；
//  2. 单价取 play 服务端配置，LockGashapon 锁内重算 gross==单价×count 强校验（6004 复用），
//     客户端金额一律不信；
//  3. 特别奖判定确定性：seed=gid（SHA256 派生），同 gid 重放结果一致可核对。
//
// Submit 立即返回 pending，前端轮询 /gashapon/records?requestId= 直到出记录。
func (l *GashaponBuyLogic) GashaponBuy(req *types.GashaponBuyReq) (*types.GashaponBuyResp, error) {
	if req.Count <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "抽数非法")
	}

	flow := &saga.BuyFlow{
		Ctx:       l.ctx,
		Log:       l.Logger,
		Svc:       l.svcCtx,
		RequestID: l.requestId,
		Scene:     "GASHAPON",
		ConfigID:  req.ConfigId,
		FailMsg:   "扭蛋请求提交失败，请稍后重试",
		// 总价 = 单价 × count（单价在 LoadUnitFen 里取服务端配置）
		AmountFen: func(unitFen int64) int64 { return unitFen * req.Count },
		LoadUnitFen: func(ctx context.Context, userID int64) (int64, error) {
			detail, err := l.svcCtx.PlayRpc.GetGashapon(ctx, &playClient.GetGashaponReq{ConfigId: req.ConfigId})
			if err != nil {
				l.Errorf("GashaponBuy GetGashapon failed, configId: %d, err: %v", req.ConfigId, err)
				return 0, errorx.FromError(err)
			}
			unitFen, err := utils.YuanToFen(detail.Price)
			if err != nil || unitFen <= 0 {
				return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "扭蛋价格无效")
			}
			return unitFen, nil
		},
		Idempotent: func(ctx context.Context, gid string, userID int64) (bool, error) {
			rec, err := l.svcCtx.PlayRpc.ListGashaponRecords(ctx, &playClient.ListGashaponRecordsReq{
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
		},
		Recheck: func(ctx context.Context, gid string, userID int64) error {
			resp, err := l.svcCtx.PlayRpc.ListGashaponRecords(ctx, &playClient.ListGashaponRecordsReq{
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
	l.Infof("gashapon buy submitted, gid: %s, configId: %d, userId: %d, count: %d, status: %s",
		gid, req.ConfigId, userID, req.Count, status)
	return &types.GashaponBuyResp{RequestId: gid, Status: status}, nil
}
