// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCouponPoolEntryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCouponPoolEntryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCouponPoolEntryLogic {
	return &GetCouponPoolEntryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCouponPoolEntry 透传 marketing-rpc（查看券池记录）。
func (l *GetCouponPoolEntryLogic) GetCouponPoolEntry(in *marketing.GetCouponPoolEntryRequest) (*marketing.CouponPoolEntry, error) {
	return l.svcCtx.MarketingRpc.GetCouponPoolEntry(l.ctx, in)
}
