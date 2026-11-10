package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListCouponTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCouponTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCouponTemplatesLogic {
	return &ListCouponTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListCouponTemplates 券模板列表（领取页）：已发布模板 + 该用户是否已领过（未核销）。
func (l *ListCouponTemplatesLogic) ListCouponTemplates(in *marketing.ListCouponTemplatesReq) (*marketing.ListCouponTemplatesResp, error) {
	if in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}
	var templates []gamemodel.CouponTemplates
	if err := l.svcCtx.DB.Where("status = ?", templatePublished).
		Order("id ASC").Find(&templates).Error; err != nil {
		l.Errorf("ListCouponTemplates query failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	items := make([]*marketing.CouponTemplateItem, 0, len(templates))
	for _, tpl := range templates {
		item := &marketing.CouponTemplateItem{
			Id:         tpl.Id,
			TemplateNo: tpl.TemplateNo,
			Name:       tpl.Name,
			Kind:       tpl.Kind,
			Value:      tpl.Value,
			MinGross:   tpl.MinGross,
			Status:     tpl.Status,
		}
		// 已领过 = 存在未核销（AVAILABLE/LOCKED）的该模板券
		var claimed int64
		l.svcCtx.DB.Model(&gamemodelnc.UserCoupons{}).
			Where("user_id = ? AND template_no = ? AND status IN ?", in.UserId, tpl.TemplateNo, []int64{couponAvailable, couponLocked}).
			Count(&claimed)
		item.Claimed = claimed > 0
		items = append(items, item)
	}
	return &marketing.ListCouponTemplatesResp{Items: items}, nil
}
