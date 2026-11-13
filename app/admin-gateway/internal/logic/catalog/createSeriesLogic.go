// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateSeriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSeriesLogic {
	return &CreateSeriesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateSeriesLogic) CreateSeries(req *catalogpb.CreateSeriesRequest) (*catalogpb.Series, error) {
	return l.svcCtx.CatalogRpc.CreateSeries(l.ctx, req)
}
