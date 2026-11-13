// Code scaffolded by goctl. Safe to edit.

package catalog

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	catalogpb "gacha_srv/app/catalog/catalog"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSeriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSeriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSeriesLogic {
	return &ListSeriesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListSeriesLogic) ListSeries(req *catalogpb.ListSeriesRequest) (*catalogpb.ListSeriesReply, error) {
	return l.svcCtx.CatalogRpc.ListSeries(l.ctx, req)
}
