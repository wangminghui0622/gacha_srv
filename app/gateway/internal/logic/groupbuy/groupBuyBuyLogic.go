package groupbuy

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

type GroupBuyBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	requestId string
}

func NewGroupBuyBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GroupBuyBuyLogic {
	return &GroupBuyBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *GroupBuyBuyLogic) SetRequestId(id string) {
	l.requestId = id
}

// GroupBuyBuy 拼团抽下注：网关发起 DTM SAGA 事务（同步预检资格+风控后 5 分支）。
//
//	[01 play.LockGroupBuy 锁报价+下注追加+余位-count → 02 money.AuthorizeWalletPayment 建单冻结
//	 → 03 play.SettleGroupBuy 落记录+余位归零时整批结算 → 04 play.GrantGroupBuyPrizes 发结算奖
//	 → 05 money.CaptureWalletPayment 捕获核销]
//	补偿：[ReleaseGroupBuy 弹注+余位+count+释放报价 → CancelPayment →
//	      SettleGroupBuyRevert 撤销记录/结算奖/lucky_sent/条目库存 → 空 → 空]
func (l *GroupBuyBuyLogic) GroupBuyBuy(req *types.GroupBuyBuyReq) (*types.GroupBuyBuyResp, error) {
	if req.Count <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "下注数非法")
	}

	flow := &saga.BuyFlow{
		Ctx:       l.ctx,
		Log:       l.Logger,
		Svc:       l.svcCtx,
		RequestID: l.requestId,
		Scene:     "GROUP_BUY",
		ConfigID:  req.ConfigId,
		FailMsg:   "拼团抽请求提交失败，请稍后重试",
		AmountFen: func(unitFen int64) int64 { return unitFen * req.Count },
		LoadUnitFen: func(ctx context.Context, userID int64) (int64, error) {
			detail, err := l.svcCtx.PlayRpc.GetGroupBuy(ctx, &playClient.GetGroupBuyReq{
				ConfigId: req.ConfigId,
				UserId:   userID,
			})
			if err != nil {
				l.Errorf("GroupBuyBuy GetGroupBuy failed, configId: %d, err: %v", req.ConfigId, err)
				return 0, errorx.FromError(err)
			}
			unitFen, err := utils.YuanToFen(detail.Price)
			if err != nil || unitFen <= 0 {
				return 0, errorx.NewCodeError(errorx.CodeAmountInvalid, "拼团抽价格无效")
			}
			return unitFen, nil
		},
		Idempotent: func(ctx context.Context, gid string, userID int64) (bool, error) {
			rec, err := l.svcCtx.PlayRpc.ListGroupBuyRecords(ctx, &playClient.ListGroupBuyRecordsReq{
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
				// 01 锁报价+下注追加+余位-count；补偿 ReleaseGroupBuy 弹注+余位+count+释放报价
				LockPath:    c.PlayRpcDirect + "/play.Play/LockGroupBuy",
				ReleasePath: c.PlayRpcDirect + "/play.Play/ReleaseGroupBuy",
				LockPayload: &playClient.LockGroupBuyReq{
					RequestId:     gid,
					QuoteToken:    quote.QuoteToken,
					ConfigId:      req.ConfigId,
					UserId:        userID,
					Count:         req.Count,
					TotalPriceFen: grossFen,
				},
				// 03 落记录+余位归零时整批结算；补偿 SettleGroupBuyRevert
				SettlePath:       c.PlayRpcDirect + "/play.Play/SettleGroupBuy",
				SettleRevertPath: c.PlayRpcDirect + "/play.Play/SettleGroupBuyRevert",
				SettlePayload: &playClient.SettleGroupBuyReq{
					RequestId: gid,
					UserId:    userID,
					ConfigId:  req.ConfigId,
					Count:     req.Count,
					Price:     utils.FenToYuan(quote.PayableFen),
				},
				// 04 发结算奖：永不 Aborted、空补偿（奖品先于捕获且不追回）
				GrantPath:       c.PlayRpcDirect + "/play.Play/GrantGroupBuyPrizes",
				GrantRevertPath: c.PlayRpcDirect + "/play.Play/GrantGroupBuyPrizesRevert",
				GrantPayload: &playClient.GrantGroupBuyPrizesReq{
					RequestId: gid,
					UserId:    userID,
					ConfigId:  req.ConfigId,
				},
			}), nil
		},
		Recheck: func(ctx context.Context, gid string, userID int64) error {
			resp, err := l.svcCtx.PlayRpc.ListGroupBuyRecords(ctx, &playClient.ListGroupBuyRecordsReq{
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
	l.Infof("group buy submitted, gid: %s, configId: %d, userId: %d, count: %d, status: %s",
		gid, req.ConfigId, userID, req.Count, status)
	return &types.GroupBuyBuyResp{RequestId: gid, Status: status}, nil
}
