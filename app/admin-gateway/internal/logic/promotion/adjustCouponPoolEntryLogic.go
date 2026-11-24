// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdjustCouponPoolEntryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdjustCouponPoolEntryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdjustCouponPoolEntryLogic {
	return &AdjustCouponPoolEntryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AdjustCouponPoolEntry 透传 marketing-rpc（管理员增减券池库存）。
func (l *AdjustCouponPoolEntryLogic) AdjustCouponPoolEntry(in *marketing.AdjustCouponPoolEntryRequest) (*marketing.CouponPoolEntry, error) {
	return l.svcCtx.MarketingRpc.AdjustCouponPoolEntry(l.ctx, in)
}
