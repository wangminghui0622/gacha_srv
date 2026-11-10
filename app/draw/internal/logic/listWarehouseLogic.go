package logic

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/svc"
	drawmodelnc "gacha_srv/pkg/model_nocache/gacha_draw"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWarehouseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWarehouseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWarehouseLogic {
	return &ListWarehouseLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListWarehouse 吉仓列表（按用户、倒序；nocache 直查）
func (l *ListWarehouseLogic) ListWarehouse(in *draw.ListWarehouseReq) (*draw.ListWarehouseResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID无效")
	}
	var items []drawmodelnc.DrawWarehouseItems
	if err := l.svcCtx.DB.WithContext(l.ctx).Where("user_id = ? AND status = ?", in.UserId, warehouseStatusIn).
		Order("id DESC").Limit(500).Find(&items).Error; err != nil {
		l.Errorf("ListWarehouse failed, userId: %d, err: %v", in.UserId, err)
		return nil, errorx.FromMySQLError(err)
	}
	names, err := boxNames(l.ctx, l.svcCtx.DB)
	if err != nil {
		l.Errorf("ListWarehouse boxNames failed, err: %v", err)
		return nil, errorx.FromMySQLError(err)
	}

	out := make([]*draw.WarehouseItem, 0, len(items))
	for _, it := range items {
		out = append(out, &draw.WarehouseItem{
			ItemId:    it.Id,
			RecordId:  it.RecordId,
			BoxId:     it.BoxId,
			BoxName:   names[it.BoxId],
			ItemName:  it.ItemName,
			CreatedAt: it.CreatedAt.Unix(),
		})
	}
	return &draw.ListWarehouseResp{Items: out}, nil
}
