package logic

import (
	"context"
	"errors"
	"strings"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type UpsertBannerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertBannerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertBannerLogic {
	return &UpsertBannerLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertBannerLogic) UpsertBanner(in *marketing.UpsertBannerRequest) (*marketing.Banner, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "tenantId 须为 1（单租户）")
	}
	b := in.Banner
	if b == nil {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "banner 不能为空")
	}
	b.Type = strings.TrimSpace(b.Type)
	if b.Type == "" || len([]rune(b.Type)) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "请选择类型")
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" || len([]rune(b.Name)) > 128 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "Banner名称须为 1-128 个字符")
	}
	b.ImageUrl = strings.TrimSpace(b.ImageUrl)
	if b.ImageUrl == "" || len(b.ImageUrl) > 512 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "请上传图片")
	}
	if b.Position < 0 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "位置不能为负数")
	}
	if b.TargetPoolId < 0 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "目标赏池ID不能为负数")
	}
	b.PopupType = strings.TrimSpace(b.PopupType)
	b.PopupTargetType = strings.TrimSpace(b.PopupTargetType)
	b.PopupTargetId = strings.TrimSpace(b.PopupTargetId)
	if len([]rune(b.PopupType)) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "弹窗类型不能超过 64 个字符")
	}
	if len([]rune(b.PopupTargetType)) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "弹窗目标类型不能超过 64 个字符")
	}
	if len(b.PopupTargetId) > 255 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "弹窗目标ID不能超过 255 个字符")
	}
	row := repo.BannerFromProto(b)
	if b.Id == 0 {
		if err := repo.InsertBanner(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	} else {
		existing, err := repo.GetBanner(l.svcCtx.DB, int64(b.Id))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewCodeError(errorx.CodeBannerNotFound, "Banner 不存在")
			}
			return nil, err
		}
		row.ID = existing.ID
		if err := repo.UpdateBanner(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	}
	saved, err := repo.GetBanner(l.svcCtx.DB, row.ID)
	if err != nil {
		return nil, err
	}
	return saved.ToProto(), nil
}
