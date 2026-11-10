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

type ListCouponTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListCouponTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCouponTemplatesLogic {
	return &ListCouponTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListCouponTemplates 可领券模板列表（券中心领取页）
func (l *ListCouponTemplatesLogic) ListCouponTemplates() (resp *types.CouponTemplateListResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.MarketingRpc.ListCouponTemplates(l.ctx, &marketingClient.ListCouponTemplatesReq{UserId: userId})
	if err != nil {
		l.Errorf("ListCouponTemplates failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.CouponTemplateItem, 0, len(r.Items))
	for _, t := range r.Items {
		items = append(items, types.CouponTemplateItem{
			Id:         t.Id,
			TemplateNo: t.TemplateNo,
			Name:       t.Name,
			Kind:       t.Kind,
			Value:      t.Value,
			MinGross:   utils.FenToYuan(t.MinGross),
			Claimed:    t.Claimed,
		})
	}
	return &types.CouponTemplateListResp{List: items}, nil
}
