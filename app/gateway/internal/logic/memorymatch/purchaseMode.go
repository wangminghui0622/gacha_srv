package memorymatch

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

// purchaseMode 记忆翻牌购买策略：实现 saga.PurchaseMode[*types.MemoryMatchPurchaseReq]。
// 幂等快路径与购买后会话轮询在 buy logic 中保留（需要返回完整 session）。
type purchaseMode struct {
	log logx.Logger
}

var _ saga.PurchaseMode[*types.MemoryMatchPurchaseReq] = purchaseMode{}

func (m purchaseMode) Name() string                                      { return "memorymatch" }
func (m purchaseMode) Scene() string                                     { return "MEMORY_MATCH" }
func (m purchaseMode) ConfigID(req *types.MemoryMatchPurchaseReq) int64  { return req.ConfigId }
func (m purchaseMode) RequireRequestID() bool                            { return true }
func (m purchaseMode) UserCouponID(req *types.MemoryMatchPurchaseReq) int64 { return req.UserCouponId }
func (m purchaseMode) FailMsg() string                                   { return "记忆翻牌请求提交失败，请稍后重试" }

func (m purchaseMode) LoadUnitFen(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MemoryMatchPurchaseReq, userID int64) (int64, error) {
	detail, err := svcCtx.PlayRpc.GetMemoryMatchConfig(ctx, &playClient.GetMemoryMatchConfigReq{
		ConfigId: req.ConfigId,
		UserId:   userID,
	})
	if err != nil {
		m.log.Errorf("MemoryMatchBuy GetMemoryMatchConfig failed, configId: %d, err: %v", req.ConfigId, err)
		return 0, errorx.FromError(err)
	}
	unitFen, err := utils.YuanToFen(detail.Config.PurchasePrice)
	if err != nil || unitFen <= 0 {
		return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "记忆翻牌价格无效")
	}
	return unitFen, nil
}

func (m purchaseMode) AmountFen(unitFen int64, req *types.MemoryMatchPurchaseReq) int64 { return unitFen }

func (m purchaseMode) Prepare(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MemoryMatchPurchaseReq, userID int64) error {
	return nil
}

// Idempotent 幂等快路径在 MemoryMatchBuy 中单独实现（需要返回完整 session），这里不重复查询。
func (m purchaseMode) Idempotent(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MemoryMatchPurchaseReq, gid string, userID int64) (bool, error) {
	return false, nil
}

func (m purchaseMode) BuildBranches(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MemoryMatchPurchaseReq,
	gid string, userID int64, quote *marketingClient.QuoteCheckoutResp, unitFen, grossFen int64) ([]saga.Branch, error) {
	c := svcCtx.Config
	return saga.PlayBranches(c.MoneyRpcDirect, gid, userID, req.ConfigId, quote, saga.PlayBranchSet{
		// 01 锁报价+会话插入 PENDING；补偿 LockMemoryMatchRevert 会话 EXPIRED+释放报价
		LockPath:    c.PlayRpcDirect + "/play.Play/LockMemoryMatch",
		ReleasePath: c.PlayRpcDirect + "/play.Play/LockMemoryMatchRevert",
		LockPayload: &playClient.LockMemoryMatchReq{
			RequestId:     gid,
			QuoteToken:    quote.QuoteToken,
			ConfigId:      req.ConfigId,
			UserId:        userID,
			FacePresetId:  req.FacePresetId,
			BackPresetId:  req.BackPresetId,
			TotalPriceFen: unitFen,
		},
		// 03 PENDING→AWAITING_DESTINY；补偿 SettleMemoryMatchRevert
		SettlePath:       c.PlayRpcDirect + "/play.Play/SettleMemoryMatch",
		SettleRevertPath: c.PlayRpcDirect + "/play.Play/SettleMemoryMatchRevert",
		SettlePayload: &playClient.SettleMemoryMatchReq{
			RequestId: gid,
			UserId:    userID,
			ConfigId:  req.ConfigId,
			SessionId: 0,
			Price:     utils.FenToYuan(quote.PayableFen),
		},
		// 04 发奖空操作（购买时无奖，终局结算同步发奖；永不 Aborted、空补偿）
		GrantPath:       c.PlayRpcDirect + "/play.Play/GrantMemoryMatchPrizes",
		GrantRevertPath: c.PlayRpcDirect + "/play.Play/GrantMemoryMatchPrizesRevert",
		GrantPayload: &playClient.GrantMemoryMatchPrizesReq{
			RequestId: gid,
			UserId:    userID,
			SessionId: 0,
			ConfigId:  req.ConfigId,
		},
	}), nil
}

// Recheck Submit 失败由 buy logic 的会话轮询兜底，这里返回 ErrRecordNotFound。
func (m purchaseMode) Recheck(ctx context.Context, svcCtx *svc.ServiceContext, req *types.MemoryMatchPurchaseReq, gid string, userID int64) error {
	return saga.ErrRecordNotFound
}
