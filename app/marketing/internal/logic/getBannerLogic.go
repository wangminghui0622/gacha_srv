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

type GetBannerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBannerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBannerLogic {
	return &GetBannerLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetBannerLogic) GetBanner(in *marketing.GetBannerRequest) (*marketing.Banner, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "tenantId 须为 1（单租户）")
	}
	if in.Id == 0 {
		return nil, errorx.NewCodeError(errorx.CodeBannerInvalid, "id 不能为空")
	}
	row, err := repo.GetBanner(l.svcCtx.DB, int64(in.Id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeBannerNotFound, "Banner 不存在")
		}
		return nil, err
	}
	return row.ToProto(), nil
}
