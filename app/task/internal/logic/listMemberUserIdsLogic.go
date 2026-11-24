package logic

import (
	"context"

	"gacha_srv/app/task/internal/repo"
	"gacha_srv/app/task/internal/svc"
	"gacha_srv/app/task/task"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	listMemberUserIDsDefaultLimit = 100
	listMemberUserIDsMaxLimit     = 1000
)

type ListMemberUserIdsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMemberUserIdsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemberUserIdsLogic {
	return &ListMemberUserIdsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListMemberUserIds 按累计消费区间分页列出会员用户ID（控制台批量发券人群筛选）。
func (l *ListMemberUserIdsLogic) ListMemberUserIds(in *task.ListMemberUserIdsReq) (*task.ListMemberUserIdsResp, error) {
	if in.MinSpendMinor < 0 || in.MaxSpendMinor < 0 || in.AfterUserId < 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "消费区间/游标不能为负数")
	}
	if in.MaxSpendMinor > 0 && in.MaxSpendMinor < in.MinSpendMinor {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "最高消费不能小于最低消费")
	}
	limit := int(in.Limit)
	if limit <= 0 {
		limit = listMemberUserIDsDefaultLimit
	}
	if limit > listMemberUserIDsMaxLimit {
		limit = listMemberUserIDsMaxLimit
	}
	ids, next, err := repo.ListMemberUserIDs(l.ctx, l.svcCtx.DB, in.MinSpendMinor, in.MaxSpendMinor, in.AfterUserId, limit)
	if err != nil {
		return nil, err
	}
	return &task.ListMemberUserIdsResp{UserIds: ids, NextAfterUserId: next}, nil
}
