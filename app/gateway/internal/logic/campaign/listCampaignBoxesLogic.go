package campaign

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	marketingClient "gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCampaignBoxesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListCampaignBoxesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCampaignBoxesLogic {
	return &ListCampaignBoxesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListCampaignBoxes 我的活动宝箱资格（boxName 由 draw 箱列表补齐展示名）
func (l *ListCampaignBoxesLogic) ListCampaignBoxes() (resp *types.EntitlementListResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.MarketingRpc.ListCampaignBoxEntitlements(l.ctx, &marketingClient.ListCampaignBoxEntitlementsReq{UserId: userId})
	if err != nil {
		l.Errorf("ListCampaignBoxes failed, err: %v", err)
		return nil, errorx.FromError(err)
	}

	// boxId → 箱名（draw 箱列表一次取全；缺失按空名）
	boxNames := map[int64]string{}
	if boxes, berr := l.svcCtx.DrawRpc.ListBoxes(l.ctx, &drawClient.ListBoxesReq{}); berr == nil {
		for _, b := range boxes.Boxes {
			boxNames[b.BoxId] = b.BoxName
		}
	}
	resp = &types.EntitlementListResp{List: make([]types.EntitlementItem, 0, len(r.Items))}
	for _, it := range r.Items {
		resp.List = append(resp.List, types.EntitlementItem{
			EntitlementNo: it.EntitlementNo,
			BoxId:         it.BoxId,
			BoxName:       boxNames[it.BoxId],
			Status:        it.Status,
			GrantedAt:     it.GrantedAt,
		})
	}
	return resp, nil
}
