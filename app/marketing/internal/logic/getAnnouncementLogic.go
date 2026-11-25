package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetAnnouncementLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAnnouncementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAnnouncementLogic {
	return &GetAnnouncementLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetAnnouncementLogic) GetAnnouncement(in *marketing.GetAnnouncementRequest) (*marketing.Announcement, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "tenantId 须为 1（单租户）")
	}
	if in.Id == 0 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "id 不能为空")
	}
	row, err := repo.GetAnnouncement(l.svcCtx.DB, int64(in.Id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeAnnouncementNotFound, "公告不存在")
		}
		return nil, err
	}
	return row.ToProto(), nil
}
