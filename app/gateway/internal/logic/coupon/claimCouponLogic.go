package coupon
import (
	"context"

	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClaimCouponLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClaimCouponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimCouponLogic {
	return &ClaimCouponLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ClaimCoupon 领取券：领取幂等键由网关生成（重发=网络重试场景同键）
func (l *ClaimCouponLogic) ClaimCoupon(req *types.ClaimCouponReq) (resp *types.ClaimCouponResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.MarketingRpc.ClaimCoupon(l.ctx, &marketingClient.ClaimCouponReq{
		UserId:         userId,
		TemplateNo:     req.TemplateNo,
		ClaimRequestId: uuid.New().String(),
	})
	if err != nil {
		l.Errorf("ClaimCoupon failed, templateNo: %s, err: %v", req.TemplateNo, err)
		return nil, errorx.FromError(err)
	}
	return &types.ClaimCouponResp{CouponNo: r.CouponNo}, nil
}
