package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProductLogic {
	return &GetProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查看商品（ACTIVE 读已发布快照，否则读 live 草稿表；无校验，未找到=4001，照抄 server_v2）
func (l *GetProductLogic) GetProduct(in *catalog.GetProductRequest) (*catalog.Product, error) {
	out, err := l.svcCtx.Repo.Find(l.ctx, in.ProductId)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
