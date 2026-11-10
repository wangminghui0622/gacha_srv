package draw

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type BoxQuoteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBoxQuoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BoxQuoteLogic {
	return &BoxQuoteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// BoxQuote 购买页报价：宝箱原价取 draw 服务端定价，券/积分抵扣由 game 服务算
// （客户端只传选择项，价格永远服务端算）。每次调用生成新报价（OPEN 锁，15 分钟 TTL），
// 购买提交时会以购买 gid 重新报价——本端点纯 UX 预览。
func (l *BoxQuoteLogic) BoxQuote(req *types.BoxQuoteReq) (resp *types.BoxQuoteResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}

	detail, err := l.svcCtx.DrawRpc.BoxDetail(l.ctx, &drawClient.BoxDetailReq{BoxId: req.BoxId})
	if err != nil {
		l.Errorf("BoxQuote BoxDetail failed, boxId: %d, err: %v", req.BoxId, err)
		return nil, errorx.FromError(err)
	}
	if detail.SoldOut {
		return nil, errorx.NewCodeError(errorx.CodeBoxSoldOut, "宝箱已售罄")
	}
	grossFen, err := utils.YuanToFen(detail.Price)
	if err != nil || grossFen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAmountInvalid, "宝箱价格无效")
	}

	gid := uuid.New().String()
	quote, err := l.svcCtx.MarketingRpc.QuoteCheckout(l.ctx, &marketingClient.QuoteCheckoutReq{
		RequestId:        gid,
		UserId:           userId,
		BoxId:            req.BoxId,
		UserCouponId:     req.UserCouponId,
		UseLoyaltyOffset: req.UseLoyaltyOffset,
		GrossFen:         grossFen,
	})
	if err != nil {
		l.Errorf("BoxQuote QuoteCheckout failed, boxId: %d, err: %v", req.BoxId, err)
		return nil, errorx.FromError(err)
	}

	return &types.BoxQuoteResp{
		RequestId:          gid,
		QuoteToken:         quote.QuoteToken,
		Gross:              detail.Price,
		CouponDiscount:     utils.FenToYuan(quote.CouponDiscountFen),
		CouponNo:           quote.CouponNo,
		LoyaltyPointsDebit: quote.LoyaltyPointsDebit,
		Loyalty:            utils.FenToYuan(quote.LoyaltyFen),
		Payable:            utils.FenToYuan(quote.PayableFen),
		Cash:               utils.FenToYuan(quote.CashFen),
		LoyaltyBalance:     utils.FenToYuan(quote.LoyaltyPointBalance),
		LoyaltyRate:        quote.LoyaltyOffsetRate,
		LoyaltyMax:         utils.FenToYuan(quote.LoyaltyMaxFen),
	}, nil
}
