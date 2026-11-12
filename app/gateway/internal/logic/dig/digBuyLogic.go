package dig

import (
	"context"
	"fmt"

	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DigBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入）
	requestId string
}

func NewDigBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DigBuyLogic {
	return &DigBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *DigBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// DigBuy 挖宝购买：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支）。
//
//	[01 play.LockDigBox 锁报价+会话位图 → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleDigBox 揭晓+落记录 → 04 play.GrantDigPrizes 发奖（永不 Aborted，空补偿）
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseDigBox 清位回滚+释放报价 → CancelPayment →
//	      SettleDigBoxRevert 撤销记录 → 空 → 空]
//
// 安全不变式：
//  1. userId 取 JWT 上下文，不接受客户端传入；
//  2. 单价取 play 服务端配置，格子数网关解析（takeAll=会话空位），
//     LockDigBox 锁内重算 gross==单价×格数 强校验（6018），客户端金额一律不信；
//  3. 幂等快路径必须在格子解析之前——全开后重放时 resolveDigCells 会先报 6002。
func (l *DigBuyLogic) DigBuy(req *types.DigBuyReq) (*types.DigBuyResp, error) {
	// Prepare 解析出的本轮格子与 sessionNo，供 AmountFen / BuildBranches 使用。
	var (
		cells     []int64
		sessionNo string
	)

	flow := &saga.BuyFlow{
		Ctx:       l.ctx,
		Log:       l.Logger,
		Svc:       l.svcCtx,
		RequestID: l.requestId,
		Scene:     "DIG_BOX",
		ConfigID:  req.BoxId,
		FailMsg:   "挖宝请求提交失败，请稍后重试",
		LoadUnitFen: func(ctx context.Context, userID int64) (int64, error) {
			boxes, err := l.svcCtx.PlayRpc.ListDigBoxes(ctx, &playClient.ListDigBoxesReq{})
			if err != nil {
				l.Errorf("DigBuy ListDigBoxes failed, err: %v", err)
				return 0, errorx.FromError(err)
			}
			var unitPrice string
			for _, it := range boxes.Items {
				if it.BoxId == req.BoxId {
					unitPrice = it.Price
					break
				}
			}
			if unitPrice == "" {
				return 0, errorx.NewCodeError(errorx.CodeDigBoxNotFound, "挖宝箱不存在")
			}
			unitFen, err := utils.YuanToFen(unitPrice)
			if err != nil || unitFen <= 0 {
				return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "挖宝价格无效")
			}
			return unitFen, nil
		},
		Prepare: func(ctx context.Context, userID int64) error {
			var err error
			cells, err = resolveDigCells(ctx, l.svcCtx, userID, req.BoxId, req.CellIndexes, req.TakeAll)
			if err != nil {
				return err
			}
			sessionNo = fmt.Sprintf("dg_%d_%d", userID, req.BoxId)
			return nil
		},
		AmountFen: func(unitFen int64) int64 { return int64(len(cells)) * unitFen },
		Idempotent: func(ctx context.Context, gid string, userID int64) (bool, error) {
			rec, err := l.svcCtx.PlayRpc.ListDigRecords(ctx, &playClient.ListDigRecordsReq{
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
			return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.BoxId, quote, saga.PlayBranchSet{
				// 01 锁报价+会话位图；补偿 ReleaseDigBox 清位回滚+释放报价
				LockPath:    c.PlayRpcDirect + "/play.Play/LockDigBox",
				ReleasePath: c.PlayRpcDirect + "/play.Play/ReleaseDigBox",
				LockPayload: &playClient.LockDigBoxReq{
					RequestId:     gid,
					QuoteToken:    quote.QuoteToken,
					ConfigId:      req.BoxId,
					SessionNo:     sessionNo,
					CellIndexes:   cells,
					TakeAll:       false, // 网关已把 takeAll 解析为显式格子（补偿精确回滚依据）
					TotalPriceFen: grossFen,
				},
				// 03 揭晓+落记录；补偿 SettleDigBoxRevert 撤销记录
				SettlePath:       c.PlayRpcDirect + "/play.Play/SettleDigBox",
				SettleRevertPath: c.PlayRpcDirect + "/play.Play/SettleDigBoxRevert",
				SettlePayload: &playClient.SettleDigBoxReq{
					RequestId:   gid,
					UserId:      userID,
					ConfigId:    req.BoxId,
					SessionNo:   sessionNo,
					CellIndexes: cells,
					TakeAll:     false,
					Price:       utils.FenToYuan(quote.PayableFen),
				},
				// 04 发奖：永不 Aborted、空补偿（奖品先于捕获且不追回）
				GrantPath:       c.PlayRpcDirect + "/play.Play/GrantDigPrizes",
				GrantRevertPath: c.PlayRpcDirect + "/play.Play/GrantDigPrizesRevert",
				GrantPayload: &playClient.GrantDigPrizesReq{
					RequestId: gid,
					UserId:    userID,
				},
			}), nil
		},
		Recheck: func(ctx context.Context, gid string, userID int64) error {
			resp, err := l.svcCtx.PlayRpc.ListDigRecords(ctx, &playClient.ListDigRecordsReq{
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
	l.Infof("dig buy submitted, gid: %s, boxId: %d, userId: %d, cells: %d, status: %s",
		gid, req.BoxId, userID, len(cells), status)
	return &types.DigBuyResp{RequestId: gid, Status: status}, nil
}
