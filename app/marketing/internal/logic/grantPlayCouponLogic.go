package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GrantPlayCouponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantPlayCouponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantPlayCouponLogic {
	return &GrantPlayCouponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantPlayCoupon 玩法结算发券（grantDigPrizes/memorymatch 结算 sub-call）：
// coupon_no="uc_"+requestId 确定性派生，uk 撞键 = 同单重发 → 读回原券幂等返回。
// 发券行照抄 仓玩法结算语义：AVAILABLE、无锁、无期限。
func (l *GrantPlayCouponLogic) GrantPlayCoupon(in *marketing.GrantPlayCouponReq) (*marketing.GrantPlayCouponResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.TemplateNo == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "发券参数不能为空")
	}
	couponNo := "uc_" + in.RequestId
	row := &gamemodelnc.UserCoupons{
		CouponNo:   couponNo,
		UserId:     in.UserId,
		TemplateNo: in.TemplateNo,
		Status:     couponAvailable,
		LockedBy:   "",
		ExpiresAt:  nil,
	}
	if err := l.svcCtx.CouponModel.Insert(l.ctx, l.svcCtx.DB, row); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 同单重发：读回原券幂等返回
			existing, ferr := l.svcCtx.CouponModel.FindOneByCouponNo(l.ctx, couponNo)
			if ferr != nil {
				l.Errorf("GrantPlayCoupon readback failed, couponNo: %s, err: %v", couponNo, ferr)
				return nil, ferr
			}
			return &marketing.GrantPlayCouponResp{CouponNo: existing.CouponNo}, nil
		}
		l.Errorf("GrantPlayCoupon insert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, err
	}
	return &marketing.GrantPlayCouponResp{CouponNo: couponNo}, nil
}
