package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	adminop "gacha_srv/pkg/adminop"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPermissionCatalogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPermissionCatalogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPermissionCatalogLogic {
	return &ListPermissionCatalogLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPermissionCatalog 权限目录（pkg/adminop 共享目录：资源 → 动作，含中文名）。
func (l *ListPermissionCatalogLogic) ListPermissionCatalog(in *ops.ListPermissionCatalogRequest) (*ops.ListPermissionCatalogReply, error) {
	resources := adminop.CatalogResources()
	reply := &ops.ListPermissionCatalogReply{}
	for _, r := range resources {
		item := &ops.PermissionCatalogResource{Code: r.Code, Name: r.Name}
		for _, action := range r.Actions {
			item.Actions = append(item.Actions, &ops.PermissionCatalogAction{Code: action, Name: adminop.ActionName(action)})
		}
		reply.Resources = append(reply.Resources, item)
	}
	return reply, nil
}
