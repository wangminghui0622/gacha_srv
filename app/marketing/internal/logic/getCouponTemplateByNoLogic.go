package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCouponTemplateByNoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCouponTemplateByNoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCouponTemplateByNoLogic {
	return &GetCouponTemplateByNoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCouponTemplateByNo 按模板单号查券模板（玩法发奖展示券名；NotFound 调用方静默降级）
func (l *GetCouponTemplateByNoLogic) GetCouponTemplateByNo(in *marketing.GetCouponTemplateByNoReq) (*marketing.CouponTemplateItem, error) {
	if in.TemplateNo == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "模板单号不能为空")
	}
	tpl, err := l.svcCtx.TemplateModel.FindOneByTemplateNo(l.ctx, in.TemplateNo)
	if err != nil {
		if errors.Is(err, gamemodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeCouponTemplateNotFound, "券模板不存在")
		}
		l.Errorf("GetCouponTemplateByNo failed, templateNo: %s, err: %v", in.TemplateNo, err)
		return nil, errorx.FromMySQLError(err)
	}
	return &marketing.CouponTemplateItem{
		Id:         tpl.Id,
		TemplateNo: tpl.TemplateNo,
		Name:       tpl.Name,
		Kind:       tpl.Kind,
		Value:      tpl.Value,
		MinGross:   tpl.MinGross,
		Status:     tpl.Status,
	}, nil
}
