// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFortuneRewardsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFortuneRewardsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFortuneRewardsLogic {
	return &ListFortuneRewardsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListFortuneRewardsLogic) ListFortuneRewards(in *marketing.ListFortuneRewardsRequest) (*marketing.ListFortuneRewardsReply, error) {
	return l.svcCtx.MarketingRpc.ListFortuneRewards(l.ctx, in)
}
