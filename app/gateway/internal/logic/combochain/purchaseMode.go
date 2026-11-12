package combochain

import (
	"context"

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

// purchaseMode 连击链购买策略：实现 saga.PurchaseMode[*types.ComboChainBuyReq]。
type purchaseMode struct {
	log logx.Logger
}

var _ saga.PurchaseMode[*types.ComboChainBuyReq] = purchaseMode{}

func (m purchaseMode) Name() string                                  { return "combochain" }
func (m purchaseMode) Scene() string                                 { return "COMBO_CHAIN" }
func (m purchaseMode) ConfigID(req *types.ComboChainBuyReq) int64    { return req.ConfigId }
func (m purchaseMode) RequireRequestID() bool                        { return false }
func (m purchaseMode) UserCouponID(req *types.ComboChainBuyReq) int64 { return 0 }
func (m purchaseMode) FailMsg() string                               { return "连击链请求提交失败，请稍后重试" }

func (m purchaseMode) LoadUnitFen(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ComboChainBuyReq, userID int64) (int64, error) {
	detail, err := svcCtx.PlayRpc.GetComboChain(ctx, &playClient.GetComboChainReq{
		ConfigId: req.ConfigId,
		UserId:   userID,
	})
	if err != nil {
		m.log.Errorf("ComboChainBuy GetComboChain failed, configId: %d, err: %v", req.ConfigId, err)
		return 0, errorx.FromError(err)
	}
	unitFen, err := utils.YuanToFen(detail.Price)
	if err != nil || unitFen <= 0 {
		return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "连击链价格无效")
	}
	return unitFen, nil
}

func (m purchaseMode) AmountFen(unitFen int64, req *types.ComboChainBuyReq) int64 { return unitFen }

func (m purchaseMode) Prepare(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ComboChainBuyReq, userID int64) error {
	return nil
}

func (m purchaseMode) Idempotent(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ComboChainBuyReq, gid string, userID int64) (bool, error) {
	rec, err := playapi.ListComboChainRecords(ctx, svcCtx, gid, userID, 0, 0)
	if err != nil {
		return false, err
	}
	return rec != nil && len(rec.Items) > 0, nil
}

func (m purchaseMode) BuildBranches(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ComboChainBuyReq,
	gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
	c := svcCtx.Config
	lockBytes, _ := proto.Marshal(&playClient.LockComboChainReq{
		RequestId:     gid,
		QuoteToken:    quote.QuoteToken,
		ConfigId:      req.ConfigId,
		UserId:        userID,
		TotalPriceFen: unitFen,
	})
	settleBytes, _ := proto.Marshal(&playClient.SettleComboChainReq{
		RequestId: gid,
		UserId:    userID,
		ConfigId:  req.ConfigId,
		Price:     utils.FenToYuan(quote.PayableFen),
	})
	grantBytes, _ := proto.Marshal(&playClient.GrantComboChainPrizesReq{
		RequestId: gid,
		UserId:    userID,
		ConfigId:  req.ConfigId,
	})
	return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.ConfigId, quote, saga.PlayBranchSet{
		LockPath:    c.PlayRpcDirect + saga.PathPlayLock,
		ReleasePath: c.PlayRpcDirect + saga.PathPlayRelease,
		LockPayload: &play.PlayLockReq{
			RequestId:     gid,
			Mode:          play.PlayMode_PLAY_MODE_COMBO_CHAIN,
			ConfigId:      req.ConfigId,
			UserId:        userID,
			QuoteToken:    quote.QuoteToken,
			TotalPriceFen: unitFen,
			Payload:       lockBytes,
		},
		SettlePath:       c.PlayRpcDirect + saga.PathPlaySettle,
		SettleRevertPath: c.PlayRpcDirect + saga.PathPlaySettleRevert,
		SettlePayload: &play.PlaySettleReq{
			RequestId: gid,
			Mode:      play.PlayMode_PLAY_MODE_COMBO_CHAIN,
			ConfigId:  req.ConfigId,
			UserId:    userID,
			Price:     utils.FenToYuan(quote.PayableFen),
			Payload:   settleBytes,
		},
		GrantPath:       c.PlayRpcDirect + saga.PathPlayGrant,
		GrantRevertPath: c.PlayRpcDirect + saga.PathPlayGrantRevert,
		GrantPayload: &play.PlayGrantReq{
			RequestId: gid,
			Mode:      play.PlayMode_PLAY_MODE_COMBO_CHAIN,
			ConfigId:  req.ConfigId,
			UserId:    userID,
			Payload:   grantBytes,
		},
	}), nil
}

func (m purchaseMode) Recheck(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ComboChainBuyReq, gid string, userID int64) error {
	resp, err := playapi.ListComboChainRecords(ctx, svcCtx, gid, userID, 0, 0)
	if err != nil {
		return err
	}
	if resp == nil || len(resp.Items) == 0 {
		return saga.ErrRecordNotFound
	}
	return nil
}
