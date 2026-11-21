package logic

import (
	"context"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMemoryMatchAppearancesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMemoryMatchAppearancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemoryMatchAppearancesLogic {
	return &ListMemoryMatchAppearancesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListMemoryMatchAppearancesLogic) ListMemoryMatchAppearances(in *play.ListMemoryMatchAppearancesReq) (*play.ListMemoryMatchAppearancesResp, error) {
	page, pageSize := mmNormalizePage(in.GetPage(), in.GetPageSize())
	typeFilter := in.GetType()
	if typeFilter == 0 {
		typeFilter = -1
	}
	store := repo.NewMemoryMatchAdminStore(l.svcCtx.DB)
	rows, total, err := store.ListAppearances(l.ctx, typeFilter, in.GetStatus(), in.GetKeyword(), page, pageSize)
	if err != nil {
		return nil, mmDBError(err)
	}
	items := make([]*play.MemoryMatchAppearanceItem, 0, len(rows))
	for i := range rows {
		items = append(items, mmAppearanceToProto(&rows[i]))
	}
	return &play.ListMemoryMatchAppearancesResp{Items: items, Total: total}, nil
}
