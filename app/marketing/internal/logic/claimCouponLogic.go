package logic

import (
	"context"
	"errors"
	"fmt"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type ClaimCouponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimCouponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimCouponLogic {
	return &ClaimCouponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClaimCoupon 领取券：coupon_no="uc_"+claimRequestId 确定性派生，
// uk 撞键 = 重发幂等（读回原券号返回，不重复发放）。
func (l *ClaimCouponLogic) ClaimCoupon(in *marketing.ClaimCouponReq) (*marketing.ClaimCouponResp, error) {
	if in.UserId <= 0 || in.TemplateNo == "" || in.ClaimRequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}
	couponNo := "uc_" + in.ClaimRequestId

	// ① 幂等：券号已存在直接读回（任意状态）
	if existing, err := l.svcCtx.CouponModel.FindOneByCouponNo(l.ctx, couponNo); err == nil {
		l.Infof("ClaimCoupon idempotent read-back, couponNo: %s, status: %d", couponNo, existing.Status)
		return &marketing.ClaimCouponResp{CouponNo: couponNo}, nil
	} else if !errors.Is(err, gamemodelnc.ErrNotFound) {
		l.Errorf("ClaimCoupon read-back query failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ② 模板校验：存在且已发布
	tpl, err := l.svcCtx.TemplateModel.FindOneByTemplateNo(l.ctx, in.TemplateNo)
	if errors.Is(err, gamemodel.ErrNotFound) || (err == nil && tpl.Status != templatePublished) {
		return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=券模板不存在或未发布", errorx.CodeCouponTemplateNotFound))
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ③ 发放（uk coupon_no；并发撞键 → 读回）
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
			l.Infof("ClaimCoupon duplicate key, read-back, couponNo: %s", couponNo)
			return &marketing.ClaimCouponResp{CouponNo: couponNo}, nil
		}
		l.Errorf("ClaimCoupon insert failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("ClaimCoupon ok, couponNo: %s, templateNo: %s, userId: %d", couponNo, in.TemplateNo, in.UserId)
	return &marketing.ClaimCouponResp{CouponNo: couponNo}, nil
}
