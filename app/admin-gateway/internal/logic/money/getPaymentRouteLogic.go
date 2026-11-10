// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package money

import (
	"context"

	"gacha_srv/app/money/money"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPaymentRouteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPaymentRouteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPaymentRouteLogic {
	return &GetPaymentRouteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetPaymentRoute 透传 money-rpc（tenant 固定 1；无行 → 8003）。
func (l *GetPaymentRouteLogic) GetPaymentRoute(in *money.GetPaymentRouteRequest) (*money.GetPaymentRouteReply, error) {
	return l.svcCtx.MoneyRpc.GetPaymentRoute(l.ctx, in)
}
