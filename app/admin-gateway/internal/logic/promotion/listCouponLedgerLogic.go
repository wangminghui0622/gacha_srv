// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCouponLedgerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCouponLedgerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCouponLedgerLogic {
	return &ListCouponLedgerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListCouponLedger 透传 marketing-rpc（券池/用户券流水）。
func (l *ListCouponLedgerLogic) ListCouponLedger(in *marketing.ListCouponLedgerRequest) (*marketing.ListCouponLedgerReply, error) {
	return l.svcCtx.MarketingRpc.ListCouponLedger(l.ctx, in)
}
