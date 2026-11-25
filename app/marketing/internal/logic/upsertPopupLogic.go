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

type UpsertPopupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertPopupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertPopupLogic {
	return &UpsertPopupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertPopupLogic) UpsertPopup(in *marketing.UpsertPopupRequest) (*marketing.Popup, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "tenantId 须为 1（单租户）")
	}
	p := in.Popup
	if p == nil {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "popup 不能为空")
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len([]rune(p.Name)) > 128 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "名称须为 1-128 个字符")
	}
	p.Status = strings.TrimSpace(p.Status)
	if p.Status == "" {
		p.Status = "未开启"
	}
	if len([]rune(p.Status)) > 16 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "状态不能超过 16 个字符")
	}
	if p.SortOrder < 0 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "序号不能为负数")
	}
	p.Type = strings.TrimSpace(p.Type)
	p.Condition = strings.TrimSpace(p.Condition)
	p.JumpType = strings.TrimSpace(p.JumpType)
	p.JumpTarget = strings.TrimSpace(p.JumpTarget)
	p.ImageUrl = strings.TrimSpace(p.ImageUrl)
	if len([]rune(p.Type)) > 64 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "类型不能超过 64 个字符")
	}
	if len([]rune(p.Condition)) > 64 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "条件不能超过 64 个字符")
	}
	if len([]rune(p.JumpType)) > 64 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "跳转类型不能超过 64 个字符")
	}
	if len(p.JumpTarget) > 255 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "跳转目标不能超过 255 个字符")
	}
	if len(p.ImageUrl) > 512 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "弹窗图片地址不能超过 512 个字符")
	}
	row := repo.PopupFromProto(p)
	if p.Id == 0 {
		if err := repo.InsertPopup(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	} else {
		existing, err := repo.GetPopup(l.svcCtx.DB, int64(p.Id))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewCodeError(errorx.CodePopupNotFound, "弹窗不存在")
			}
			return nil, err
		}
		row.ID = existing.ID
		if err := repo.UpdatePopup(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	}
	saved, err := repo.GetPopup(l.svcCtx.DB, row.ID)
	if err != nil {
		return nil, err
	}
	return saved.ToProto(), nil
}
