package logic

import (
	"context"
	"strings"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/pagination"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBrandsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBrandsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBrandsLogic {
	return &ListBrandsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListBrands 品牌列表（分页 + 名称搜索，默认每页 15 条，最大 100）。
func (l *ListBrandsLogic) ListBrands(in *catalog.ListBrandsRequest) (*catalog.ListBrandsReply, error) {
	page, pageSize := pagination.Normalize(in.Page, in.PageSize, 15)
	rows, total, err := l.svcCtx.Repo.ListBrands(l.ctx, strings.TrimSpace(in.Keyword), page, pageSize)
	if err != nil {
		return nil, mapBrandRepoErr(err)
	}
	return &catalog.ListBrandsReply{Brands: rows, Total: total, Page: page, PageSize: pageSize}, nil
}
