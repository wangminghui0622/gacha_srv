package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProductsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProductsLogic {
	return &ListProductsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 商品列表（id 游标分页，摘要；size 默认 20 封顶 100，照抄 server_v2 biz.List）
func (l *ListProductsLogic) ListProducts(in *catalog.ListProductsRequest) (*catalog.ListProductsReply, error) {
	size := in.PageSize
	if size == 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	rows, err := l.svcCtx.Repo.List(l.ctx, in.AfterProductId, size+1)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := &catalog.ListProductsReply{}
	if len(rows) <= int(size) {
		out.Products = rows
		return out, nil
	}
	out.Products = rows[:size]
	out.NextAfterProductId = rows[size-1].ProductId // 游标=末条已回行 ID（server_v2 先截断再取 last）
	return out, nil
}
