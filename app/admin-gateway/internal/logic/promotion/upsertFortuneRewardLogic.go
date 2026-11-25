// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertFortuneRewardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertFortuneRewardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertFortuneRewardLogic {
	return &UpsertFortuneRewardLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertFortuneRewardLogic) UpsertFortuneReward(in *marketing.UpsertFortuneRewardRequest) (*marketing.FortuneReward, error) {
	return l.svcCtx.MarketingRpc.UpsertFortuneReward(l.ctx, in)
}
