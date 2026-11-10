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

type GetCouponTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCouponTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCouponTemplateLogic {
	return &GetCouponTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCouponTemplate 设计 D5：console GET 路径仅 template_id（无版本参数），取最新版。
func (l *GetCouponTemplateLogic) GetCouponTemplate(in *marketing.GetCouponTemplateRequest) (*marketing.CouponTemplate, error) {
	if in.TenantId == 0 || in.TemplateId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCouponInvalid, "tenantId/templateId 必须大于 0")
	}
	row, err := repo.GetLatestCouponTemplateVersion(l.svcCtx.DB, int64(in.TemplateId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeCouponTemplateNotFound, "券模板不存在")
		}
		return nil, err
	}
	return row.ToProto(), nil
}
