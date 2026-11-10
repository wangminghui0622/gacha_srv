package groupbuy

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupBuyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupBuyLogic {
	return &GetGroupBuyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetGroupBuy 拼团抽详情：余位/我的下注/参与人数/幸运奖进度（JWT 组，需登录）
func (l *GetGroupBuyLogic) GetGroupBuy(configId int64) (resp *types.GroupBuyDetailResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetGroupBuy(l.ctx, &playClient.GetGroupBuyReq{ConfigId: configId, UserId: userId})
	if err != nil {
		l.Errorf("GetGroupBuy failed, configId: %d, err: %v", configId, err)
		return nil, errorx.FromError(err)
	}
	lucky := make([]types.GroupBuyLuckyPrizeItem, 0, len(r.LuckyPrizes))
	for _, p := range r.LuckyPrizes {
		lucky = append(lucky, types.GroupBuyLuckyPrizeItem{
			PrizeIndex: p.PrizeIndex,
			Quantity:   p.Quantity,
			Sent:       p.Sent,
			Prize:      digPrizeItemOf(p.Prize),
		})
	}
	return &types.GroupBuyDetailResp{
		ConfigId:         r.ConfigId,
		Name:             r.Name,
		Price:            r.Price,
		TargetBetCount:   r.TargetBetCount,
		PerBetAwardLimit: r.PerBetAwardLimit,
		RemainingBets:    r.RemainingBets,
		MyBets:           r.MyBets,
		Participants:     r.Participants,
		Status:           r.Status,
		LuckyPrizes:      lucky,
	}, nil
}
