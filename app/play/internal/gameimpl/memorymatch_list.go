package gameimpl

import (
	"context"
	"encoding/json"
	"strings"

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
	// 只返回真正上线的对对碰赏池（play_pool_catalog.status=LIVE），
	// 避免把仅用于联调/种子的 memory_match_configs 记录暴露给玩家。
	if err := l.svcCtx.DB.Table("memory_match_configs AS c").
		Select("c.*").
		Joins("JOIN play_pool_catalog AS p ON p.id = c.id").
		Where("c.status = ? AND p.mode = ? AND p.status = ?", 1, "memorymatch", 2).
		Order("c.id DESC").Limit(100).Find(&rows).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "读取对对碰主题失败")
	}
	ids := make([]int64, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].Id)
	}
	type poolExtraRow struct {
		ID        int64  `gorm:"column:id"`
		AdminJSON string `gorm:"column:admin_json"`
	}
	var extras []poolExtraRow
	if len(ids) > 0 {
		_ = l.svcCtx.DB.Table("play_pool_catalog").Select("id, admin_json").
			Where("id IN ?", ids).Find(&extras).Error
	}
	covers := make(map[int64]string, len(extras))
	for _, extra := range extras {
		var meta struct {
			CoverImage string `json:"coverImage"`
			CoverThumb string `json:"coverThumb"`
		}
		if json.Unmarshal([]byte(extra.AdminJSON), &meta) != nil {
			continue
		}
		if strings.TrimSpace(meta.CoverImage) != "" {
			covers[extra.ID] = meta.CoverImage
		} else if strings.TrimSpace(meta.CoverThumb) != "" {
			covers[extra.ID] = meta.CoverThumb
		}
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
			CoverImage:        covers[row.Id],
		})
	}
	return resp, nil
}
