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

// purchaseMode 挖宝购买策略：实现 saga.PurchaseMode[*types.DigBuyReq]。
// Prepare 解析出的格子/sessionNo 保存在 mode 上，供 AmountFen/BuildBranches 使用。
type purchaseMode struct {
	log       logx.Logger
	cells     []int64
	sessionNo string
}

var _ saga.PurchaseMode[*types.DigBuyReq] = (*purchaseMode)(nil)

func (m *purchaseMode) Name() string                     { return "dig" }
func (m *purchaseMode) Scene() string                    { return "DIG_BOX" }
func (m *purchaseMode) ConfigID(req *types.DigBuyReq) int64 { return req.BoxId }
func (m *purchaseMode) RequireRequestID() bool           { return false }
func (m *purchaseMode) UserCouponID(req *types.DigBuyReq) int64 { return 0 }
func (m *purchaseMode) FailMsg() string                  { return "挖宝请求提交失败，请稍后重试" }

func (m *purchaseMode) LoadUnitFen(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DigBuyReq, userID int64) (int64, error) {
	boxes, err := svcCtx.PlayRpc.ListDigBoxes(ctx, &playClient.ListDigBoxesReq{})
	if err != nil {
		m.log.Errorf("DigBuy ListDigBoxes failed, err: %v", err)
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
}

func (m *purchaseMode) AmountFen(unitFen int64, req *types.DigBuyReq) int64 {
	return int64(len(m.cells)) * unitFen
}

func (m *purchaseMode) Prepare(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DigBuyReq, userID int64) error {
	cells, err := resolveDigCells(ctx, svcCtx, userID, req.BoxId, req.CellIndexes, req.TakeAll)
	if err != nil {
		return err
	}
	m.cells = cells
	m.sessionNo = fmt.Sprintf("dg_%d_%d", userID, req.BoxId)
	return nil
}

func (m *purchaseMode) Idempotent(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DigBuyReq, gid string, userID int64) (bool, error) {
	rec, err := svcCtx.PlayRpc.ListDigRecords(ctx, &playClient.ListDigRecordsReq{
		RequestId: gid,
		UserId:    userID,
	})
	if err != nil {
		return false, err
	}
	return rec != nil && len(rec.Items) > 0, nil
}

func (m *purchaseMode) BuildBranches(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DigBuyReq,
	gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
	c := svcCtx.Config
	return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.BoxId, quote, saga.PlayBranchSet{
		// 01 锁报价+会话位图；补偿 ReleaseDigBox 清位回滚+释放报价
		LockPath:    c.PlayRpcDirect + "/play.Play/LockDigBox",
		ReleasePath: c.PlayRpcDirect + "/play.Play/ReleaseDigBox",
		LockPayload: &playClient.LockDigBoxReq{
			RequestId:     gid,
			QuoteToken:    quote.QuoteToken,
			ConfigId:      req.BoxId,
			SessionNo:     m.sessionNo,
			CellIndexes:   m.cells,
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
			SessionNo:   m.sessionNo,
			CellIndexes: m.cells,
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
}

func (m *purchaseMode) Recheck(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DigBuyReq, gid string, userID int64) error {
	resp, err := svcCtx.PlayRpc.ListDigRecords(ctx, &playClient.ListDigRecordsReq{
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
