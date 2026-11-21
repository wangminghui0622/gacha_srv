package logic

import (
	"context"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteMemoryMatchDeckTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteMemoryMatchDeckTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMemoryMatchDeckTemplateLogic {
	return &DeleteMemoryMatchDeckTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteMemoryMatchDeckTemplateLogic) DeleteMemoryMatchDeckTemplate(in *play.DeleteMemoryMatchDeckTemplateReq) (*play.DeleteMemoryMatchDeckTemplateResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "牌堆 ID 无效")
	}
	store := repo.NewMemoryMatchAdminStore(l.svcCtx.DB)
	if err := store.DeleteDeckTemplate(l.ctx, in.Id); err != nil {
		return nil, mmDBError(err)
	}
	return &play.DeleteMemoryMatchDeckTemplateResp{Deleted: true}, nil
}
