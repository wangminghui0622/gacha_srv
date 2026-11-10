package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProductLogic {
	return &CreateProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建商品（DRAFT + version 1；选项/SKU 全量校验，照抄 server_v2 buildProduct）
func (l *CreateProductLogic) CreateProduct(in *catalog.CreateProductRequest) (*catalog.Product, error) {
	draft, err := core.BuildProduct(in)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品校验失败："+err.Error())
	}
	out, err := l.svcCtx.Repo.Save(l.ctx, draft)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
