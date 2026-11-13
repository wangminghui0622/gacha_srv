package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateBrandLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateBrandLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateBrandLogic {
	return &CreateBrandLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateBrand 创建品牌。
func (l *CreateBrandLogic) CreateBrand(in *catalog.CreateBrandRequest) (*catalog.Brand, error) {
	name, logo, err := core.BuildBrand(in.Name, in.Logo)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, err.Error())
	}
	out, err := l.svcCtx.Repo.CreateBrand(l.ctx, name, logo)
	if err != nil {
		return nil, mapBrandRepoErr(err)
	}
	return out, nil
}
