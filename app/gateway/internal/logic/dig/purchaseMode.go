package dig

import (
	"context"
	"fmt"

	"gacha_srv/app/gateway/internal/saga"
	"gacha_srv/app/gateway/internal/playapi"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
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
	rec, err := playapi.ListDigRecords(ctx, svcCtx, gid, userID, 0, 0)
	if err != nil {
		return false, err
	}
	return rec != nil && len(rec.Items) > 0, nil
}

func (m *purchaseMode) BuildBranches(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DigBuyReq,
	gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
	c := svcCtx.Config
	lockBytes, _ := proto.Marshal(&playClient.LockDigBoxReq{
		RequestId:     gid,
		QuoteToken:    quote.QuoteToken,
		ConfigId:      req.BoxId,
		SessionNo:     m.sessionNo,
		CellIndexes:   m.cells,
		TakeAll:       false,
		TotalPriceFen: grossFen,
	})
	settleBytes, _ := proto.Marshal(&playClient.SettleDigBoxReq{
		RequestId:   gid,
		UserId:      userID,
		ConfigId:    req.BoxId,
		SessionNo:   m.sessionNo,
		CellIndexes: m.cells,
		TakeAll:     false,
		Price:       utils.FenToYuan(quote.PayableFen),
	})
	grantBytes, _ := proto.Marshal(&playClient.GrantDigPrizesReq{
		RequestId: gid,
		UserId:    userID,
	})
	return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.BoxId, quote, saga.PlayBranchSet{
		LockPath:    c.PlayRpcDirect + saga.PathPlayLock,
		ReleasePath: c.PlayRpcDirect + saga.PathPlayRelease,
		LockPayload: &play.PlayLockReq{
			RequestId:     gid,
			Mode:          play.PlayMode_PLAY_MODE_DIG_BOX,
			ConfigId:      req.BoxId,
			UserId:        userID,
			QuoteToken:    quote.QuoteToken,
			TotalPriceFen: grossFen,
			Payload:       lockBytes,
		},
		SettlePath:       c.PlayRpcDirect + saga.PathPlaySettle,
		SettleRevertPath: c.PlayRpcDirect + saga.PathPlaySettleRevert,
		SettlePayload: &play.PlaySettleReq{
			RequestId: gid,
			Mode:      play.PlayMode_PLAY_MODE_DIG_BOX,
			ConfigId:  req.BoxId,
			UserId:    userID,
			Price:     utils.FenToYuan(quote.PayableFen),
			Payload:   settleBytes,
		},
		GrantPath:       c.PlayRpcDirect + saga.PathPlayGrant,
		GrantRevertPath: c.PlayRpcDirect + saga.PathPlayGrantRevert,
		GrantPayload: &play.PlayGrantReq{
			RequestId: gid,
			Mode:      play.PlayMode_PLAY_MODE_DIG_BOX,
			ConfigId:  req.BoxId,
			UserId:    userID,
			Payload:   grantBytes,
		},
	}), nil
}

func (m *purchaseMode) Recheck(ctx context.Context, svcCtx *svc.ServiceContext, req *types.DigBuyReq, gid string, userID int64) error {
	resp, err := playapi.ListDigRecords(ctx, svcCtx, gid, userID, 0, 0)
	if err != nil {
		return err
	}
	if resp == nil || len(resp.Items) == 0 {
		return saga.ErrRecordNotFound
	}
	return nil
}
