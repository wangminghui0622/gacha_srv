package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishProductRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishProductRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishProductRevisionLogic {
	return &PublishProductRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 发布修订（版本前进 expected+1；已发布幂等重放；expected≥2，照抄 server_v2）
func (l *PublishProductRevisionLogic) PublishProductRevision(in *catalog.PublishProductRevisionRequest) (*catalog.Product, error) {
	if in.ProductId == 0 || in.ExpectedVersion < 2 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品与期望版本必填")
	}
	out, err := l.svcCtx.Repo.PublishRevision(l.ctx, in.ProductId, in.ExpectedVersion)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
