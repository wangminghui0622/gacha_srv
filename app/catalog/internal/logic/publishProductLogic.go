package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishProductLogic {
	return &PublishProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 首次发布（DRAFT→ACTIVE，激活 SKU，版本前进落快照；已发布幂等返回，照抄 server_v2）
func (l *PublishProductLogic) PublishProduct(in *catalog.PublishProductRequest) (*catalog.Product, error) {
	if in.ProductId == 0 || in.ExpectedVersion == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品与期望版本必填")
	}
	out, err := l.svcCtx.Repo.Publish(l.ctx, in.ProductId, in.ExpectedVersion)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
