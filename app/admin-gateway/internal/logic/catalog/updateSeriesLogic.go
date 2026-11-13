// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSeriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSeriesLogic {
	return &UpdateSeriesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateSeriesLogic) UpdateSeries(req *catalogpb.UpdateSeriesRequest) (*catalogpb.Series, error) {
	return l.svcCtx.CatalogRpc.UpdateSeries(l.ctx, req)
}
