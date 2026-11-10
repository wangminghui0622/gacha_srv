package logic

import (
	"context"
	"errors"
	"strings"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveProductRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveProductRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveProductRevisionLogic {
	return &SaveProductRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 保存修订（改标题/SKU 字段；草稿 blob；同内容幂等；校验照抄 server_v2 biz.SaveRevision）
func (l *SaveProductRevisionLogic) SaveProductRevision(in *catalog.SaveProductRevisionRequest) (*catalog.Product, error) {
	if in.ProductId == 0 || in.ExpectedVersion == 0 || strings.TrimSpace(in.Title) == "" || len(in.Skus) == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "商品、期望版本、标题与 SKU 必填")
	}
	current, err := l.svcCtx.Repo.Find(l.ctx, in.ProductId)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	draft, err := core.MakeRevisionDraft(current, in.ExpectedVersion, in.Title, in.Skus)
	if err != nil {
		if errors.Is(err, core.ErrVariantConflict) {
			return nil, mapVariantErr(err)
		}
		return nil, errorx.FromError(err)
	}
	out, err := l.svcCtx.Repo.SaveRevision(l.ctx, in.ProductId, in.ExpectedVersion, draft)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return out, nil
}
