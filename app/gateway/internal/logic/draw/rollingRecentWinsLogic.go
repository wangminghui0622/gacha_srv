package draw

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RollingRecentWinsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRollingRecentWinsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RollingRecentWinsLogic {
	return &RollingRecentWinsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// RollingRecentWins 滚动中奖 feed（公开，首页 marquee 轮询）
func (l *RollingRecentWinsLogic) RollingRecentWins(req *types.RollingRecentWinsReq) (*types.RollingRecentWinsResp, error) {
	r, err := l.svcCtx.DrawRpc.ListRollingDrawRecentWins(l.ctx, &drawClient.ListRollingDrawRecentWinsReq{
		Limit:       req.Limit,
		MarqueeOnly: req.MarqueeOnly,
		BoxId:       req.BoxId,
	})
	if err != nil {
		l.Errorf("RollingRecentWins failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	wins := make([]types.RollingWinItem, 0, len(r.Wins))
	for _, it := range r.Wins {
		wins = append(wins, types.RollingWinItem{
			RequestId:       it.RequestId,
			UserId:          it.UserId,
			BoxId:           it.BoxId,
			EntryName:       it.EntryName,
			DisplayValueFen: it.DisplayValueFen,
			Tier:            it.Tier,
			WonAt:           it.WonAt,
		})
	}
	return &types.RollingRecentWinsResp{
		Wins:                  wins,
		HighValueThresholdFen: r.HighValueThresholdFen,
	}, nil
}
