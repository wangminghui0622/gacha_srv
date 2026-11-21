package memorymatch

import (
	"context"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMemoryMatchConfigsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMemoryMatchConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemoryMatchConfigsLogic {
	return &ListMemoryMatchConfigsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListMemoryMatchConfigs 玩家端对对碰主题列表。
func (l *ListMemoryMatchConfigsLogic) ListMemoryMatchConfigs() (resp *types.MemoryMatchConfigListResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.ListMemoryMatchConfigs(l.ctx, &playClient.ListMemoryMatchConfigsReq{})
	if err != nil {
		l.Errorf("ListMemoryMatchConfigs failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	resp = &types.MemoryMatchConfigListResp{Configs: make([]types.MemoryMatchConfigSummary, 0, len(r.Configs))}
	for _, c := range r.Configs {
		if c == nil {
			continue
		}
		resp.Configs = append(resp.Configs, types.MemoryMatchConfigSummary{
			ConfigId:          c.ConfigId,
			Name:              c.Name,
			PurchasePrice:     c.PurchasePrice,
			RoundsPerPurchase: c.RoundsPerPurchase,
			MaxPairCount:      c.MaxPairCount,
			Status:            c.Status,
			CoverImage:        c.CoverImage,
		})
	}
	return resp, nil
}
