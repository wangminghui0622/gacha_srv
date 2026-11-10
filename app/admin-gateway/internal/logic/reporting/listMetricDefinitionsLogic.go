// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package reporting

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
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

// ListMetricDefinitions 透传 ops-rpc（reporting.metric:read）。
func (l *ListMetricDefinitionsLogic) ListMetricDefinitions(in *ops.ListMetricDefinitionsRequest) (*ops.ListMetricDefinitionsReply, error) {
	return l.svcCtx.OpsRpc.ListMetricDefinitions(l.ctx, in)
}
