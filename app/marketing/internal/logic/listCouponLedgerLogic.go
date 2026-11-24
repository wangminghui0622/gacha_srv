package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	listCouponLedgerDefaultPageSize = 20
	listCouponLedgerMaxPageSize     = 100
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

// ListCouponLedger 券池库存/用户券状态变动流水（id 倒序）。
func (l *ListCouponLedgerLogic) ListCouponLedger(in *marketing.ListCouponLedgerRequest) (*marketing.ListCouponLedgerReply, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId 须为 1（单租户）")
	}
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listCouponLedgerDefaultPageSize
	}
	if pageSize > listCouponLedgerMaxPageSize {
		pageSize = listCouponLedgerMaxPageSize
	}
	rows, total, err := repo.ListCouponLedger(l.svcCtx.DB,
		int64(in.EntryId), int64(in.UserCouponId), int64(in.UserId), in.OrderId,
		(page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	entries := make([]*marketing.CouponLedgerEntry, 0, len(rows))
	for i := range rows {
		entries = append(entries, rows[i].ToProto())
	}
	return &marketing.ListCouponLedgerReply{Entries: entries, Total: uint64(total)}, nil
}
