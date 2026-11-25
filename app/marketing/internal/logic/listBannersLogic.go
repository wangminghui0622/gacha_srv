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
	listBannersDefaultPageSize = 15
	listBannersMaxPageSize     = 100
)

type ListBannersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBannersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBannersLogic {
	return &ListBannersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListBannersLogic) ListBanners(in *marketing.ListBannersRequest) (*marketing.ListBannersReply, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "tenantId 须为 1（单租户）")
	}
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listBannersDefaultPageSize
	}
	if pageSize > listBannersMaxPageSize {
		pageSize = listBannersMaxPageSize
	}
	rows, total, err := repo.ListBanners(l.svcCtx.DB, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*marketing.Banner, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToProto())
	}
	return &marketing.ListBannersReply{Items: items, Total: uint64(total)}, nil
}
