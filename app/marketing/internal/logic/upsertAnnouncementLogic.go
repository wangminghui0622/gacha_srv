package logic

import (
	"context"
	"errors"
	"html"
	"regexp"
	"strings"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

var announcementHTMLTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

type UpsertAnnouncementLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertAnnouncementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertAnnouncementLogic {
	return &UpsertAnnouncementLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertAnnouncementLogic) UpsertAnnouncement(in *marketing.UpsertAnnouncementRequest) (*marketing.Announcement, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "tenantId 须为 1（单租户）")
	}
	a := in.Announcement
	if a == nil {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "announcement 不能为空")
	}
	a.Title = strings.TrimSpace(a.Title)
	if a.Title == "" || len([]rune(a.Title)) > 255 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "公告标题须为 1-255 个字符")
	}
	if a.Type < 0 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "公告类型无效")
	}
	if a.PublishAtUnixMillis <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "请选择公告日期")
	}
	a.JumpType = strings.TrimSpace(a.JumpType)
	a.JumpTarget = strings.TrimSpace(a.JumpTarget)
	if len([]rune(a.JumpType)) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "跳转类型不能超过 32 个字符")
	}
	if len([]rune(a.JumpTarget)) > 255 {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "跳转目标不能超过 255 个字符")
	}
	plain := strings.TrimSpace(html.UnescapeString(announcementHTMLTagPattern.ReplaceAllString(a.Content, "")))
	plain = strings.ReplaceAll(plain, "\u00a0", "")
	if strings.TrimSpace(plain) == "" {
		return nil, errorx.NewCodeError(errorx.CodeAnnouncementInvalid, "请输入公告内容")
	}
	row := repo.AnnouncementFromProto(a)
	if a.Id == 0 {
		if err := repo.InsertAnnouncement(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	} else {
		existing, err := repo.GetAnnouncement(l.svcCtx.DB, int64(a.Id))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewCodeError(errorx.CodeAnnouncementNotFound, "公告不存在")
			}
			return nil, err
		}
		row.ID = existing.ID
		if err := repo.UpdateAnnouncement(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	}
	saved, err := repo.GetAnnouncement(l.svcCtx.DB, row.ID)
	if err != nil {
		return nil, err
	}
	return saved.ToProto(), nil
}
