package logic

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPublishedPoolsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPublishedPoolsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPublishedPoolsLogic {
	return &ListPublishedPoolsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPublishedPools 已发布赏池列表（server_v2 ListPoolVersions 的 published 视图：
// 仅 PUBLISHED 版本，游标分页，返回完整草稿配置）。
func (l *ListPublishedPoolsLogic) ListPublishedPools(in *draw.ListPublishedPoolsReq) (*draw.ListPublishedPoolsResp, error) {
	pageSize := clampPageSize(in.GetPageSize())
	rows, err := repo.ListPublishedPools(l.svcCtx.DB, in.GetAfterPoolId(), in.GetAfterVersion(), pageSize+1)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}

	resp := &draw.ListPublishedPoolsResp{}
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		last := rows[len(rows)-1]
		resp.NextAfterPoolId = last.PoolID
		resp.NextAfterVersion = last.Version
	}
	for i := range rows {
		box, err := poolDecode(&rows[i])
		if err != nil {
			return nil, err
		}
		resp.Pools = append(resp.Pools, box)
	}
	return resp, nil
}
