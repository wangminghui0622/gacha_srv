// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFortuneRewardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFortuneRewardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFortuneRewardLogic {
	return &GetFortuneRewardLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetFortuneRewardLogic) GetFortuneReward(in *marketing.GetFortuneRewardRequest) (*marketing.FortuneReward, error) {
	return l.svcCtx.MarketingRpc.GetFortuneReward(l.ctx, in)
}
