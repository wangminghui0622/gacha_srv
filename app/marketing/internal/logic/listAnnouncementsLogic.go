package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	listAnnouncementsDefaultPageSize = 15
	listAnnouncementsMaxPageSize     = 100
)

type ListAnnouncementsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAnnouncementsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAnnouncementsLogic {
	return &ListAnnouncementsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListAnnouncementsLogic) ListAnnouncements(in *marketing.ListAnnouncementsRequest) (*marketing.ListAnnouncementsReply, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "tenantId 须为 1（单租户）")
	}
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listAnnouncementsDefaultPageSize
	}
	if pageSize > listAnnouncementsMaxPageSize {
		pageSize = listAnnouncementsMaxPageSize
	}
	rows, total, err := repo.ListAnnouncements(l.svcCtx.DB, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*marketing.Announcement, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToProto())
	}
	return &marketing.ListAnnouncementsReply{Items: items, Total: uint64(total)}, nil
}
