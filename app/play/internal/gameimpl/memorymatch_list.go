package gameimpl

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMemoryMatchConfigsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMemoryMatchConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemoryMatchConfigsLogic {
	return &ListMemoryMatchConfigsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListMemoryMatchConfigs 玩家端对对碰主题列表：只返回上架配置，最新创建的在前。
func (l *ListMemoryMatchConfigsLogic) ListMemoryMatchConfigs(_ *play.ListMemoryMatchConfigsReq) (*play.ListMemoryMatchConfigsResp, error) {
	var rows []gamemodel.MemoryMatchConfigs
	if err := l.svcCtx.DB.Where("status = ?", 1).Order("id DESC").Limit(100).Find(&rows).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "读取对对碰主题失败")
	}
	resp := &play.ListMemoryMatchConfigsResp{Configs: make([]*play.MemoryMatchConfigSummary, 0, len(rows))}
	for i := range rows {
		row := &rows[i]
		resp.Configs = append(resp.Configs, &play.MemoryMatchConfigSummary{
			ConfigId:          row.Id,
			Name:              row.Name,
			PurchasePrice:     utils.FenToYuan(row.PurchasePriceMinor),
			RoundsPerPurchase: row.RoundsPerPurchase,
			MaxPairCount:      row.MaxPairCount,
			Status:            row.Status,
		})
	}
	return resp, nil
}
