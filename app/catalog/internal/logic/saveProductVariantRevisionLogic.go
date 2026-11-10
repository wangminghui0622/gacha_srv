package logic

import (
	"context"
	"errors"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveProductVariantRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveProductVariantRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveProductVariantRevisionLogic {
	return &SaveProductVariantRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 保存变体修订（增删规格值/SKU；稳定 SKU 组合不可变；校验照抄 server_v2 biz.SaveVariantRevision）
func (l *SaveProductVariantRevisionLogic) SaveProductVariantRevision(in *catalog.SaveProductVariantRevisionRequest) (*catalog.Product, error) {
	if in.ProductId == 0 || in.ExpectedVersion < 2 {
		return nil, mapVariantErr(nil)
	}
	current, err := l.svcCtx.Repo.Find(l.ctx, in.ProductId)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if current.Status != catalog.ProductStatus_PRODUCT_STATUS_ACTIVE || current.Version != in.ExpectedVersion ||
		in.Product.ProductCode != current.ProductCode {
		return nil, mapVariantErr(nil)
	}
	draft, err := core.BuildProduct(in.Product)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品校验失败："+err.Error())
	}
	stored, err := core.MakeVariantRevision(draft, current)
	if err != nil {
		if errors.Is(err, core.ErrVariantConflict) {
			return nil, mapVariantErr(err)
		}
		return nil, errorx.FromError(err)
	}
	out, err := l.svcCtx.Repo.SaveRevision(l.ctx, in.ProductId, in.ExpectedVersion, stored)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
