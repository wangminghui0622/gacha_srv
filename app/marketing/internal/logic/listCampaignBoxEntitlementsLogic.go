package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListCampaignBoxEntitlementsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCampaignBoxEntitlementsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCampaignBoxEntitlementsLogic {
	return &ListCampaignBoxEntitlementsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListCampaignBoxEntitlements 我的活动宝箱资格列表（boxName 由网关用 draw 服务补齐展示名）
func (l *ListCampaignBoxEntitlementsLogic) ListCampaignBoxEntitlements(in *marketing.ListCampaignBoxEntitlementsReq) (*marketing.ListCampaignBoxEntitlementsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var rows []gamemodelnc.CampaignBoxEntitlements
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).Order("granted_at DESC, id DESC").Find(&rows).Error; err != nil {
		l.Errorf("ListCampaignBoxEntitlements query failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	items := make([]*marketing.EntitlementItem, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		items = append(items, &marketing.EntitlementItem{
			EntitlementNo: r.EntitlementNo,
			BoxId:         r.BoxId,
			BoxName:       "",
			Status:        r.Status,
			GrantedAt:     r.GrantedAt.Unix(),
		})
	}
	return &marketing.ListCampaignBoxEntitlementsResp{Items: items}, nil
}
