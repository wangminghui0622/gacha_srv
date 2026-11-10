package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMetricDefinitionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMetricDefinitionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMetricDefinitionsLogic {
	return &ListMetricDefinitionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListMetricDefinitions 指标目录（status=1 的指标 + 维度聚合）。
func (l *ListMetricDefinitionsLogic) ListMetricDefinitions(in *ops.ListMetricDefinitionsRequest) (*ops.ListMetricDefinitionsReply, error) {
	metrics, err := l.svcCtx.ReportingStore.ListMetricDefinitions(l.ctx)
	if err != nil {
		return nil, err
	}
	return &ops.ListMetricDefinitionsReply{Metrics: metrics}, nil
}
