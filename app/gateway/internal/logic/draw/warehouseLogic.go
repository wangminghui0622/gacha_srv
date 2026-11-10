package draw

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type WarehouseLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWarehouseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WarehouseLogic {
	return &WarehouseLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Warehouse 吉仓物品列表（v1 服务端上限 500，不分页）：userId 取 JWT 上下文
func (l *WarehouseLogic) Warehouse() (resp *types.WarehouseListResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.DrawRpc.ListWarehouse(l.ctx, &drawClient.ListWarehouseReq{UserId: userId})
	if err != nil {
		l.Errorf("Warehouse failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	items := make([]types.WarehouseItem, 0, len(r.Items))
	for _, it := range r.Items {
		items = append(items, types.WarehouseItem{
			ItemId:    it.ItemId,
			RecordId:  it.RecordId,
			BoxId:     it.BoxId,
			BoxName:   it.BoxName,
			ItemName:  it.ItemName,
			CreatedAt: it.CreatedAt,
		})
	}
	return &types.WarehouseListResp{List: items}, nil
}
