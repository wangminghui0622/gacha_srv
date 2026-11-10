package coupon
import (
	"context"

	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUserCouponsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListUserCouponsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUserCouponsLogic {
	return &ListUserCouponsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListUserCoupons 我的券列表（购买页券选择器 + 券中心共用）
func (l *ListUserCouponsLogic) ListUserCoupons() (resp *types.CouponListResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.MarketingRpc.ListUserCoupons(l.ctx, &marketingClient.ListUserCouponsReq{UserId: userId})
	if err != nil {
		l.Errorf("ListUserCoupons failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.CouponItem, 0, len(r.Items))
	for _, c := range r.Items {
		items = append(items, types.CouponItem{
			Id:         c.Id,
			CouponNo:   c.CouponNo,
			TemplateNo: c.TemplateNo,
			Name:       c.Name,
			Kind:       c.Kind,
			Value:      c.Value,
			MinGross:   utils.FenToYuan(c.MinGross),
			Status:     c.Status,
			ExpiresAt:  c.ExpiresAt,
			Usable:     c.Usable,
		})
	}
	return &types.CouponListResp{List: items}, nil
}
