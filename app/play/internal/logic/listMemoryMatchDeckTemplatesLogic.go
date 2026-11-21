package logic

import (
	"context"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMemoryMatchDeckTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMemoryMatchDeckTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemoryMatchDeckTemplatesLogic {
	return &ListMemoryMatchDeckTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListMemoryMatchDeckTemplatesLogic) ListMemoryMatchDeckTemplates(in *play.ListMemoryMatchDeckTemplatesReq) (*play.ListMemoryMatchDeckTemplatesResp, error) {
	page, pageSize := mmNormalizePage(in.GetPage(), in.GetPageSize())
	store := repo.NewMemoryMatchAdminStore(l.svcCtx.DB)
	rows, total, err := store.ListDeckTemplates(l.ctx, in.GetKeyword(), in.GetStatus(), page, pageSize)
	if err != nil {
		return nil, mmDBError(err)
	}
	items := make([]*play.MemoryMatchDeckTemplate, 0, len(rows))
	for i := range rows {
		items = append(items, mmDeckTemplateToProto(&rows[i]))
	}
	return &play.ListMemoryMatchDeckTemplatesResp{Items: items, Total: total}, nil
}
