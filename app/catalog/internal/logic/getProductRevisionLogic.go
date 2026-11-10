package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProductRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProductRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProductRevisionLogic {
	return &GetProductRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 读取修订草稿（productId 必填；无草稿=4001，照抄 server_v2）
func (l *GetProductRevisionLogic) GetProductRevision(in *catalog.GetProductRevisionRequest) (*catalog.Product, error) {
	if in.ProductId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品必填")
	}
	out, err := l.svcCtx.Repo.FindRevision(l.ctx, in.ProductId)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
