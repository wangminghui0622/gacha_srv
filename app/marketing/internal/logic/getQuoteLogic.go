package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetQuoteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetQuoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetQuoteLogic {
	return &GetQuoteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ── 玩法域跨服务只读（play-rpc 拆分后经 RPC 读报价，不直连营销库）──
// 玩法 lock 校验/屏障跳过 readback/memorymatch 实付展示共用。
func (l *GetQuoteLogic) GetQuote(in *marketing.GetQuoteReq) (*marketing.GetQuoteResp, error) {
	if in.QuoteToken == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "报价令牌不能为空")
	}
	q, err := l.svcCtx.QuoteModel.FindOneByQuoteToken(l.ctx, in.QuoteToken)
	if err != nil {
		if errors.Is(err, gamemodelnc.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeQuoteNotFound, "报价不存在")
		}
		l.Errorf("GetQuote failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, errorx.FromMySQLError(err)
	}
	return &marketing.GetQuoteResp{
		QuoteToken: q.QuoteToken,
		Status:     quoteStatusName(q.Status),
		UserId:     q.UserId,
		GrossFen:   q.GrossFen,
		BoxId:      q.BoxId,
		PayableFen: q.PayableFen,
		CouponNo:   q.CouponNo,
	}, nil
}
