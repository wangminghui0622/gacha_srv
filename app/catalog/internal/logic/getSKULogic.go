package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/repo"
	"gacha_srv/app/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSKULogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSKULogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSKULogic {
	return &GetSKULogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetSKU 按 SKU ID 查询（play 域发布实物奖时校验存在性与履约方式）。
func (l *GetSKULogic) GetSKU(in *catalog.GetSKURequest) (*catalog.SKU, error) {
	if in.SkuId == 0 {
		return nil, mapRepoErr(repo.ErrNotFound)
	}
	out, err := l.svcCtx.Repo.GetSKU(l.ctx, in.SkuId)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
