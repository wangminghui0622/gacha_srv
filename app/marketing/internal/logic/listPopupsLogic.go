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
	listPopupsDefaultPageSize = 15
	listPopupsMaxPageSize     = 100
)

type ListPopupsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPopupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPopupsLogic {
	return &ListPopupsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListPopupsLogic) ListPopups(in *marketing.ListPopupsRequest) (*marketing.ListPopupsReply, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "tenantId 须为 1（单租户）")
	}
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listPopupsDefaultPageSize
	}
	if pageSize > listPopupsMaxPageSize {
		pageSize = listPopupsMaxPageSize
	}
	rows, total, err := repo.ListPopups(l.svcCtx.DB, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*marketing.Popup, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToProto())
	}
	return &marketing.ListPopupsReply{Items: items, Total: uint64(total)}, nil
}
