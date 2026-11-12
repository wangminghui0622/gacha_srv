package combochain

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

type ComboChainBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	requestId string
}

func NewComboChainBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComboChainBuyLogic {
	return &ComboChainBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *ComboChainBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// ComboChainBuy 连击链购买：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支）。
//
//	[01 play.LockComboChain 锁报价+参与追加+余票-1 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleComboChain 落记录+末票整车结算 → 04 play.GrantComboChainPrizes 发结算奖
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseComboChain 弹参与+余票+1+释放报价 → CancelPayment →
//	      SettleComboChainRevert 撤销记录/结算奖/连击快照/档位剩余 → 空 → 空]
func (l *ComboChainBuyLogic) ComboChainBuy(req *types.ComboChainBuyReq) (*types.ComboChainBuyResp, error) {
	flow := &saga.BuyFlow{
		Ctx:       l.ctx,
		Log:       l.Logger,
		Svc:       l.svcCtx,
		RequestID: l.requestId,
		Scene:     "COMBO_CHAIN",
		ConfigID:  req.ConfigId,
		FailMsg:   "连击链请求提交失败，请稍后重试",
		LoadUnitFen: func(ctx context.Context, userID int64) (int64, error) {
			detail, err := l.svcCtx.PlayRpc.GetComboChain(ctx, &playClient.GetComboChainReq{
				ConfigId: req.ConfigId,
				UserId:   userID,
			})
			if err != nil {
				l.Errorf("ComboChainBuy GetComboChain failed, configId: %d, err: %v", req.ConfigId, err)
				return 0, errorx.FromError(err)
			}
			unitFen, err := utils.YuanToFen(detail.Price)
			if err != nil || unitFen <= 0 {
				return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "连击链价格无效")
			}
			return unitFen, nil
		},
		Idempotent: func(ctx context.Context, gid string, userID int64) (bool, error) {
			rec, err := l.svcCtx.PlayRpc.ListComboChainRecords(ctx, &playClient.ListComboChainRecordsReq{
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
				// 01 锁报价+参与追加+余票-1；补偿 ReleaseComboChain 弹参与+余票+1+释放报价
				LockPath:    c.PlayRpcDirect + "/play.Play/LockComboChain",
				ReleasePath: c.PlayRpcDirect + "/play.Play/ReleaseComboChain",
				LockPayload: &playClient.LockComboChainReq{
					RequestId:     gid,
					QuoteToken:    quote.QuoteToken,
					ConfigId:      req.ConfigId,
					UserId:        userID,
					TotalPriceFen: unitFen,
				},
				// 03 落记录+末票整车结算；补偿 SettleComboChainRevert
				SettlePath:       c.PlayRpcDirect + "/play.Play/SettleComboChain",
				SettleRevertPath: c.PlayRpcDirect + "/play.Play/SettleComboChainRevert",
				SettlePayload: &playClient.SettleComboChainReq{
					RequestId: gid,
					UserId:    userID,
					ConfigId:  req.ConfigId,
					Price:     utils.FenToYuan(quote.PayableFen),
				},
				// 04 发结算奖：永不 Aborted、空补偿（奖品先于捕获且不追回）
				GrantPath:       c.PlayRpcDirect + "/play.Play/GrantComboChainPrizes",
				GrantRevertPath: c.PlayRpcDirect + "/play.Play/GrantComboChainPrizesRevert",
				GrantPayload: &playClient.GrantComboChainPrizesReq{
					RequestId: gid,
					UserId:    userID,
					ConfigId:  req.ConfigId,
				},
			}), nil
		},
		Recheck: func(ctx context.Context, gid string, userID int64) error {
			resp, err := l.svcCtx.PlayRpc.ListComboChainRecords(ctx, &playClient.ListComboChainRecordsReq{
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
	l.Infof("combo chain buy submitted, gid: %s, configId: %d, userId: %d, status: %s",
		gid, req.ConfigId, userID, status)
	return &types.ComboChainBuyResp{RequestId: gid, Status: status}, nil
}
