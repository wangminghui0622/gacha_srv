package logic

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/repo"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPoolVersionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPoolVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPoolVersionsLogic {
	return &ListPoolVersionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPoolVersions 赏池版本列表（server_v2 ListPoolVersions：含草稿，
// (pool_id, version) 复合游标分页，page_size 默认 20 上限 100）。
func (l *ListPoolVersionsLogic) ListPoolVersions(in *draw.ListPoolVersionsReq) (*draw.ListPoolVersionsResp, error) {
	pageSize := clampPageSize(in.GetPageSize())
	rows, err := repo.ListPoolVersions(l.svcCtx.DB, in.GetAfterPoolId(), in.GetAfterVersion(), pageSize+1)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}

	resp := &draw.ListPoolVersionsResp{}
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		last := rows[len(rows)-1]
		resp.NextAfterPoolId = last.PoolID
		resp.NextAfterVersion = last.Version
	}
	for i := range rows {
		resp.Versions = append(resp.Versions, &draw.PoolVersionSummary{
			PoolId:  rows[i].PoolID,
			Version: rows[i].Version,
			Status:  poolStatusFromRow(rows[i].Status),
		})
	}
	return resp, nil
}
