// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertAnnouncementLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertAnnouncementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertAnnouncementLogic {
	return &UpsertAnnouncementLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertAnnouncementLogic) UpsertAnnouncement(in *marketing.UpsertAnnouncementRequest) (*marketing.Announcement, error) {
	return l.svcCtx.MarketingRpc.UpsertAnnouncement(l.ctx, in)
}
