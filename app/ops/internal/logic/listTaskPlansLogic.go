package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
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

// ListTaskPlans 计划版本列表（cursor = after_version，next = 本页最后一行版本）。
func (l *ListTaskPlansLogic) ListTaskPlans(in *ops.ListTaskPlansRequest) (*ops.ListTaskPlansReply, error) {
	plans, next, err := l.svcCtx.PlanStore.ListTaskPlans(l.ctx, in.GetAfterVersion(), in.GetPageSize())
	if err != nil {
		return nil, err
	}
	return &ops.ListTaskPlansReply{Plans: plans, NextAfterVersion: next}, nil
}
