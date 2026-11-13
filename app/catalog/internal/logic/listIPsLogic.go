package logic

import (
	"context"
	"strings"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/pagination"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListIPsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListIPsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListIPsLogic {
	return &ListIPsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListIPs IP 列表（分页 + 名称搜索，默认每页 15 条，最大 100）。
func (l *ListIPsLogic) ListIPs(in *catalog.ListIPsRequest) (*catalog.ListIPsReply, error) {
	page, pageSize := pagination.Normalize(in.Page, in.PageSize, 15)
	rows, total, err := l.svcCtx.Repo.ListIPs(l.ctx, strings.TrimSpace(in.Keyword), page, pageSize)
	if err != nil {
		return nil, mapIPRepoErr(err)
	}
	return &catalog.ListIPsReply{Ips: rows, Total: total, Page: page, PageSize: pageSize}, nil
}
