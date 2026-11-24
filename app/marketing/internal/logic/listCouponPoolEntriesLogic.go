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
	listCouponPoolDefaultPageSize = 20
	listCouponPoolMaxPageSize     = 100
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

// ListCouponPoolEntries 券池列表（id 倒序分页，关联引用模板的最新名称）。
func (l *ListCouponPoolEntriesLogic) ListCouponPoolEntries(in *marketing.ListCouponPoolEntriesRequest) (*marketing.ListCouponPoolEntriesReply, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId 须为 1（单租户）")
	}
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listCouponPoolDefaultPageSize
	}
	if pageSize > listCouponPoolMaxPageSize {
		pageSize = listCouponPoolMaxPageSize
	}
	rows, total, err := repo.ListCouponPoolEntries(l.svcCtx.DB, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	entries := make([]*marketing.CouponPoolEntry, 0, len(rows))
	for i := range rows {
		entries = append(entries, rows[i].ToProto())
	}
	return &marketing.ListCouponPoolEntriesReply{
		Entries: entries,
		Total:   uint64(total),
	}, nil
}
