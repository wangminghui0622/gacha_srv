package logic

import (
	"context"
	"time"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type QueryDashboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryDashboardLogic {
	return &QueryDashboardLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// QueryDashboard 查询看板（server_v2 biz.QueryDashboard 逐条同款；
// M15d 起 ListBuckets 走真实桶表，同 queryKey widget 共享一次桶查询）。
func (l *QueryDashboardLogic) QueryDashboard(in *ops.QueryDashboardRequest) (*ops.QueryDashboardReply, error) {
	from, to := in.GetFrom().AsTime(), in.GetTo().AsTime()
	if err := core.ValidateDashboardQuery(in, from, to); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, err.Error())
	}
	from, to = from.UTC(), to.UTC()
	if in.GetMaxDataPoints() == 0 {
		in.MaxDataPoints = 500
	}
	d, err := l.svcCtx.ReportingStore.GetDashboard(l.ctx, in.GetTenantId(), in.GetDashboardId())
	if err != nil {
		return nil, err
	}
	if d.GetStatus() != ops.DashboardStatus_DASHBOARD_STATUS_PUBLISHED {
		return nil, errorx.NewCodeError(errorx.CodeDashboardNotPublished, "看板未发布")
	}
	key, err := core.DashboardCacheKey(d, in)
	if err != nil {
		return nil, err
	}
	if !in.GetSkipCache() {
		if cached, ok := l.svcCtx.ResultCache.Get(l.ctx, key); ok {
			return cached, nil
		}
	}
	definitions, err := l.svcCtx.ReportingStore.ListMetricDefinitions(l.ctx)
	if err != nil {
		return nil, err
	}
	catalog := make(map[string]*ops.MetricDefinition, len(definitions))
	for _, x := range definitions {
		catalog[x.GetMetricCode()] = x
	}
	result := &ops.QueryDashboardReply{
		Dashboard: d, GeneratedAt: timestamppb.New(time.Now().UTC()),
		Results: make([]*ops.WidgetQueryResult, 0, len(d.GetWidgets())),
	}
	// queryKey 去重（server_v2 同款：同指标/粒度/维度/时间窗共享一次桶查询）
	type queryKey struct {
		metric    ops.StandardMetric
		seconds   uint32
		dimension string
		from, to  int64
	}
	loaded := map[queryKey][]*ops.WidgetDataPoint{}
	for _, w := range d.GetWidgets() {
		if _, ok := catalog[w.GetMetricCode()]; !ok {
			return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "指标 "+w.GetMetricCode()+" 未启用")
		}
		seconds := core.GrainSeconds(w.GetGrain())
		if seconds == 0 {
			return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "粒度不支持")
		}
		dimensionValue := "all"
		if w.GetDimensionCode() != "all" {
			dimensionValue = in.GetDimensionValues()[w.GetDimensionCode()]
			if !core.ValidCode(dimensionValue, 256) {
				return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "维度值非法: "+w.GetDimensionCode())
			}
		}
		fromF, toF := core.FloorTime(from, seconds), core.CeilTime(to, seconds)
		truncated := false
		if int64(in.GetMaxDataPoints())*int64(seconds) < toF.Unix()-fromF.Unix() {
			fromF = toF.Add(-time.Duration(in.GetMaxDataPoints()) * time.Duration(seconds) * time.Second)
			truncated = true
		}
		qKey := queryKey{catalog[w.GetMetricCode()].GetStandardMetric(), seconds, dimensionValue, fromF.Unix(), toF.Unix()}
		buckets, ok := loaded[qKey]
		if !ok {
			buckets, err = l.svcCtx.ReportingStore.ListBuckets(l.ctx, repo.BucketQuery{
				TenantID:          in.GetTenantId(),
				ProviderCode:      in.GetProviderCode(),
				ExternalAccountID: in.GetExternalAccountId(),
				DimensionKey:      dimensionValue,
				StandardMetric:    uint64(qKey.metric),
				BucketSeconds:     uint64(seconds),
				From:              fromF,
				To:                toF,
				Limit:             int32(in.GetMaxDataPoints()),
			})
			if err != nil {
				return nil, err
			}
			loaded[qKey] = buckets
		}
		points := buckets
		if w.GetKind() == ops.WidgetKind_WIDGET_KIND_NUMBER {
			points = core.SumPoints(points, fromF)
		}
		result.Results = append(result.Results, &ops.WidgetQueryResult{
			WidgetId: w.GetWidgetId(), WidgetCode: w.GetWidgetCode(), MetricCode: w.GetMetricCode(),
			Grain: w.GetGrain(), DimensionCode: w.GetDimensionCode(), Points: points, Truncated: truncated,
		})
	}
	if !in.GetSkipCache() {
		_ = l.svcCtx.ResultCache.Set(l.ctx, key, result)
	}
	return result, nil
}
