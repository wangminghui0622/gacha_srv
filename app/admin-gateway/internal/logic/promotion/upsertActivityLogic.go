// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertActivityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertActivityLogic {
	return &UpsertActivityLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertActivityLogic) UpsertActivity(in *marketing.UpsertActivityRequest) (*marketing.Activity, error) {
	return l.svcCtx.MarketingRpc.UpsertActivity(l.ctx, in)
}
