// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package task

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/ops/ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTaskPlansLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTaskPlansLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTaskPlansLogic {
	return &ListTaskPlansLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListTaskPlans 透传 ops-rpc（task.plan:read）。
func (l *ListTaskPlansLogic) ListTaskPlans(in *ops.ListTaskPlansRequest) (*ops.ListTaskPlansReply, error) {
	return l.svcCtx.OpsRpc.ListTaskPlans(l.ctx, in)
}
