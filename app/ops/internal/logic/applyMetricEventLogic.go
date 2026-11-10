package logic

import (
	"context"

	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApplyMetricEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplyMetricEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyMetricEventLogic {
	return &ApplyMetricEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 指标事件应用（M15d：integration 指标管线接入；内部 RPC 无 HTTP 路由）
// server_v2 biz.Apply 同款守卫：event_id/tenant 缺失 → 7026。
// applied=false 属正常幂等（inbox 已处理/修订号过期），不报错——dispatcher 据此 Ack。
func (l *ApplyMetricEventLogic) ApplyMetricEvent(in *ops.ApplyMetricEventRequest) (*ops.ApplyMetricEventReply, error) {
	if in.GetEventId() == 0 || in.GetTenantId() == 0 {
		return nil, errorx.NewCodeError(errorx.CodeMetricEventInvalid, "指标事件非法（event_id/tenant 缺失）")
	}
	applied, err := l.svcCtx.ReportingStore.ApplyMetric(l.ctx, &repo.MetricEvent{
		EventID:           in.GetEventId(),
		TenantID:          in.GetTenantId(),
		ProviderCode:      in.GetProviderCode(),
		ExternalAccountID: in.GetExternalAccountId(),
		ExternalObjectID:  in.GetExternalObjectId(),
		StandardMetric:    in.GetStandardMetric(),
		BucketStart:       in.GetBucketStart().AsTime(),
		BucketSeconds:     in.GetBucketSeconds(),
		DimensionKey:      in.GetDimensionKey(),
		Value:             in.GetValue(),
		Currency:          in.GetCurrency(),
		SourceRevision:    in.GetSourceRevision(),
	})
	if err != nil {
		return nil, errorx.NewCodeErrorf(errorx.CodeServerError, "apply metric event: %v", err)
	}
	return &ops.ApplyMetricEventReply{Applied: applied}, nil
}
