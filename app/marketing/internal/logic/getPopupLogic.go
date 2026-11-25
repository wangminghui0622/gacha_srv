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

type GetPopupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPopupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPopupLogic {
	return &GetPopupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetPopupLogic) GetPopup(in *marketing.GetPopupRequest) (*marketing.Popup, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "tenantId 须为 1（单租户）")
	}
	if in.Id == 0 {
		return nil, errorx.NewCodeError(errorx.CodePopupInvalid, "id 不能为空")
	}
	row, err := repo.GetPopup(l.svcCtx.DB, int64(in.Id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodePopupNotFound, "弹窗不存在")
		}
		return nil, err
	}
	return row.ToProto(), nil
}
