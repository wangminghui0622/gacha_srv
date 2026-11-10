package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPublishedProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPublishedProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPublishedProductLogic {
	return &GetPublishedProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 读取已发布快照（任意历史版本；productId/version 必填，照抄 server_v2）
func (l *GetPublishedProductLogic) GetPublishedProduct(in *catalog.GetPublishedProductRequest) (*catalog.Product, error) {
	if in.ProductId == 0 || in.Version == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品与版本必填")
	}
	out, err := l.svcCtx.Repo.FindPublished(l.ctx, in.ProductId, in.Version)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
