package logic

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/svc"
	drawmodel "gacha_srv/pkg/model/gacha_draw"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBoxesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBoxesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBoxesLogic {
	return &ListBoxesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListBoxes 宝箱列表：静态配置、数量少，直查 MySQL（不走缓存）
func (l *ListBoxesLogic) ListBoxes(in *draw.ListBoxesReq) (*draw.ListBoxesResp, error) {
	var boxes []drawmodel.DrawBoxes
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Where("status = ?", 1).Order("id ASC").Find(&boxes).Error; err != nil {
		l.Errorf("ListBoxes failed: %v", err)
		return nil, errorx.FromMySQLError(err)
	}

	// 售罄标记：按箱统计可抽中奖条目数（remaining>0 或 quantity=-1 不限量）；
	// 一次 GROUP BY 避免逐箱查询。无条目的宝箱不在结果中 → 计 0 → 售罄。
	type row struct {
		BoxId int64
		N     int64
	}
	var rows []row
	if err := l.svcCtx.DB.WithContext(l.ctx).Model(&drawmodel.DrawPoolEntries{}).
		Select("box_id AS box_id, SUM(CASE WHEN is_prize = 1 AND (remaining > 0 OR quantity = -1) THEN 1 ELSE 0 END) AS n").
		Group("box_id").Scan(&rows).Error; err != nil {
		l.Errorf("ListBoxes sold-out stats failed: %v", err)
		return nil, errorx.FromMySQLError(err)
	}
	drawable := make(map[int64]int64, len(rows))
	for _, r := range rows {
		drawable[r.BoxId] = r.N
	}

	items := make([]*draw.BoxItem, 0, len(boxes))
	for _, b := range boxes {
		items = append(items, &draw.BoxItem{
			BoxId:       b.Id,
			BoxName:     b.BoxName,
			Price:       utils.FenToYuan(b.Price),
			Description: b.Description,
			SoldOut:     drawable[b.Id] == 0,
		})
	}
	return &draw.ListBoxesResp{Boxes: items}, nil
}
