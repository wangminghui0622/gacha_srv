package logic

import (
	"context"
	"strings"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/pagination"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSeriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSeriesLogic {
	return &ListSeriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListSeries 系列列表（分页 + 名称搜索 + 品牌筛选，默认每页 15 条，最大 100）。
func (l *ListSeriesLogic) ListSeries(in *catalog.ListSeriesRequest) (*catalog.ListSeriesReply, error) {
	page, pageSize := pagination.Normalize(in.Page, in.PageSize, 15)
	rows, total, err := l.svcCtx.Repo.ListSeries(l.ctx, strings.TrimSpace(in.Keyword), in.BrandId, page, pageSize)
	if err != nil {
		return nil, mapSeriesRepoErr(err)
	}
	return &catalog.ListSeriesReply{Series: rows, Total: total, Page: page, PageSize: pageSize}, nil
}
