package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListUserCouponsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUserCouponsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUserCouponsLogic {
	return &ListUserCouponsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListUserCoupons 我的券列表：用户券 + 模板信息拼装（usable=AVAILABLE 且未过期）。
func (l *ListUserCouponsLogic) ListUserCoupons(in *marketing.ListUserCouponsReq) (*marketing.ListUserCouponsResp, error) {
	if in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}
	var coupons []gamemodelnc.UserCoupons
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Find(&coupons).Error; err != nil {
		l.Errorf("ListUserCoupons query failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	items := make([]*marketing.CouponItem, 0, len(coupons))
	for _, c := range coupons {
		item := &marketing.CouponItem{
			Id:         c.Id,
			CouponNo:   c.CouponNo,
			TemplateNo: c.TemplateNo,
			Status:     c.Status,
		}
		tpl, err := l.svcCtx.TemplateModel.FindOneByTemplateNo(l.ctx, c.TemplateNo)
		if err != nil && !errors.Is(err, gamemodel.ErrNotFound) {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if err == nil {
			item.Name = tpl.Name
			item.Kind = tpl.Kind
			item.Value = tpl.Value
			item.MinGross = tpl.MinGross
		}
		if c.ExpiresAt != nil {
			item.ExpiresAt = c.ExpiresAt.Format("2006-01-02 15:04:05")
		}
		// usable：AVAILABLE 且未过期
		item.Usable = c.Status == couponAvailable &&
			(c.ExpiresAt == nil || time.Now().Before(*c.ExpiresAt))
		items = append(items, item)
	}
	return &marketing.ListUserCouponsResp{Items: items}, nil
}
