package logic

import (
	"context"
	"strings"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	listActivitiesDefaultPageSize = 20
	listActivitiesMaxPageSize     = 100
)

type ListActivitiesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListActivitiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListActivitiesLogic {
	return &ListActivitiesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListActivitiesLogic) ListActivities(in *marketing.ListActivitiesRequest) (*marketing.ListActivitiesReply, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "tenantId 须为 1（单租户）")
	}
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listActivitiesDefaultPageSize
	}
	if pageSize > listActivitiesMaxPageSize {
		pageSize = listActivitiesMaxPageSize
	}
	rows, total, err := repo.ListActivities(l.svcCtx.DB, strings.TrimSpace(in.Keyword), (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*marketing.Activity, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToProto())
	}
	return &marketing.ListActivitiesReply{Items: items, Total: uint64(total)}, nil
}
