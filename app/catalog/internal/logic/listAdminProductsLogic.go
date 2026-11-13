package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/pagination"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAdminProductsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAdminProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAdminProductsLogic {
	return &ListAdminProductsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ListAdminProducts 商品管理列表（筛选 + 分页）。
func (l *ListAdminProductsLogic) ListAdminProducts(in *catalog.ListAdminProductsRequest) (*catalog.ListAdminProductsReply, error) {
	page, pageSize := pagination.Normalize(in.Page, in.PageSize, 15)
	rows, total, err := l.svcCtx.Repo.ListAdminProducts(l.ctx, in, page, pageSize)
	if err != nil {
		return nil, mapAdminProductRepoErr(err)
	}
	return &catalog.ListAdminProductsReply{Products: rows, Total: total, Page: page, PageSize: pageSize}, nil
}
