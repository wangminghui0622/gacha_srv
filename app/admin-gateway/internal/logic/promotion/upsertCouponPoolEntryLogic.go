// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertCouponPoolEntryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertCouponPoolEntryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertCouponPoolEntryLogic {
	return &UpsertCouponPoolEntryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpsertCouponPoolEntry 透传 marketing-rpc（id=0 创建，id>0 编辑）。
func (l *UpsertCouponPoolEntryLogic) UpsertCouponPoolEntry(in *marketing.UpsertCouponPoolEntryRequest) (*marketing.CouponPoolEntry, error) {
	return l.svcCtx.MarketingRpc.UpsertCouponPoolEntry(l.ctx, in)
}
