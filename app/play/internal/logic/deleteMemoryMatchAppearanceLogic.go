package logic

import (
	"context"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMemoryMatchAppearanceLogic {
	return &DeleteMemoryMatchAppearanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteMemoryMatchAppearanceLogic) DeleteMemoryMatchAppearance(in *play.DeleteMemoryMatchAppearanceReq) (*play.DeleteMemoryMatchAppearanceResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "外观 ID 无效")
	}
	store := repo.NewMemoryMatchAdminStore(l.svcCtx.DB)
	if err := store.DeleteAppearance(l.ctx, in.Id); err != nil {
		return nil, mmDBError(err)
	}
	return &play.DeleteMemoryMatchAppearanceResp{Deleted: true}, nil
}
