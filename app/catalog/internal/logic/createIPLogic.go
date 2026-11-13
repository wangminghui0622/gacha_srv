package logic

import (
	"context"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/internal/core"
	"gacha_srv/app/catalog/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateIPLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateIPLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateIPLogic {
	return &CreateIPLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateIP 创建 IP。
func (l *CreateIPLogic) CreateIP(in *catalog.CreateIPRequest) (*catalog.IP, error) {
	name, sortWeight, err := core.BuildIP(in.Name, in.SortWeight)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeCatalogInvalid, err.Error())
	}
	out, err := l.svcCtx.Repo.CreateIP(l.ctx, name, sortWeight)
	if err != nil {
		return nil, mapIPRepoErr(err)
	}
	return out, nil
}
