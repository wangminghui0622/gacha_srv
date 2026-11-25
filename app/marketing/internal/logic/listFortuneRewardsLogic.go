package logic

import (
	"context"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	listFortuneRewardsDefaultPageSize = 20
	listFortuneRewardsMaxPageSize     = 100
)

type ListFortuneRewardsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFortuneRewardsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFortuneRewardsLogic {
	return &ListFortuneRewardsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListFortuneRewardsLogic) ListFortuneRewards(in *marketing.ListFortuneRewardsRequest) (*marketing.ListFortuneRewardsReply, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "tenantId 须为 1（单租户）")
	}
	page := int(in.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = listFortuneRewardsDefaultPageSize
	}
	if pageSize > listFortuneRewardsMaxPageSize {
		pageSize = listFortuneRewardsMaxPageSize
	}
	rows, total, err := repo.ListFortuneRewards(l.svcCtx.DB, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*marketing.FortuneReward, 0, len(rows))
	for i := range rows {
		items = append(items, rows[i].ToProto())
	}
	return &marketing.ListFortuneRewardsReply{Items: items, Total: uint64(total)}, nil
}
