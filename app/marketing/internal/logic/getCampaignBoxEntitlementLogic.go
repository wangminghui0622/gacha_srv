package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetCampaignBoxEntitlementLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCampaignBoxEntitlementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCampaignBoxEntitlementLogic {
	return &GetCampaignBoxEntitlementLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCampaignBoxEntitlement 单个活动宝箱资格查询（归属校验；开箱前置检查）
func (l *GetCampaignBoxEntitlementLogic) GetCampaignBoxEntitlement(in *marketing.GetCampaignBoxEntitlementReq) (*marketing.GetCampaignBoxEntitlementResp, error) {
	if in.UserId <= 0 || in.EntitlementNo == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	row, err := l.svcCtx.EntitlementModel.FindOneByEntitlementNo(l.ctx, in.EntitlementNo)
	if isNotFound(err) || (err == nil && row.UserId != in.UserId) {
		return nil, errorx.NewCodeError(errorx.CodeEntitlementNotFound, "活动宝箱资格不存在")
	}
	if err != nil {
		l.Errorf("GetCampaignBoxEntitlement query failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row.Status != entitlementAvailable {
		return nil, errorx.NewCodeError(errorx.CodeEntitlementUnavail, "活动宝箱资格不可用")
	}
	return &marketing.GetCampaignBoxEntitlementResp{
		EntitlementNo: row.EntitlementNo,
		BoxId:         row.BoxId,
		Status:        row.Status,
	}, nil
}
