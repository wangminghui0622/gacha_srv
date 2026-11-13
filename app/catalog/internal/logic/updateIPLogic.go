package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateIPLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateIPLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateIPLogic {
	return &UpdateIPLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateIP 编辑 IP 名称/排序权重。
func (l *UpdateIPLogic) UpdateIP(in *catalog.UpdateIPRequest) (*catalog.IP, error) {
	if in.IpId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, "IP ID 必填")
	}
	name, sortWeight, err := core.BuildIP(in.Name, in.SortWeight)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, err.Error())
	}
	out, err := l.svcCtx.Repo.UpdateIP(l.ctx, in.IpId, name, sortWeight)
	if err != nil {
		return nil, mapIPRepoErr(err)
	}
	return out, nil
}
