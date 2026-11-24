// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCouponPoolEntriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCouponPoolEntriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCouponPoolEntriesLogic {
	return &ListCouponPoolEntriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListCouponPoolEntries 透传 marketing-rpc（券池分页）。
func (l *ListCouponPoolEntriesLogic) ListCouponPoolEntries(in *marketing.ListCouponPoolEntriesRequest) (*marketing.ListCouponPoolEntriesReply, error) {
	return l.svcCtx.MarketingRpc.ListCouponPoolEntries(l.ctx, in)
}
