// RpcPublisher：outbox 事件 → ops-rpc ApplyMetricEvent（D2 直投，替代老项目 Kafka）。
// payload 为 thirdparty.MetricSnapshotChanged（repo 层已回填 EventId=outbox 行 id），
// 逐字段映射为 ops.ApplyMetricEventRequest；事件解码失败视为投递失败（退避重试）。
package outbox

import (
	"context"
	"fmt"

	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/app/ops/ops"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// OpsClient ops-rpc 客户端接口（本包只依赖 ApplyMetricEvent，签名对齐生成物）。
type OpsClient interface {
	ApplyMetricEvent(ctx context.Context, in *ops.ApplyMetricEventRequest, opts ...grpc.CallOption) (*ops.ApplyMetricEventReply, error)
}

// RpcPublisher 把 outbox 消息投递给 ops-rpc。
type RpcPublisher struct {
	ops OpsClient
}

// NewRpcPublisher 构建投递器。
func NewRpcPublisher(ops OpsClient) *RpcPublisher {
	return &RpcPublisher{ops: ops}
}

// Publish 解码事件并调用 ApplyMetricEvent（server_v2 KafkaPublisher.Publish 的直投替身）。
func (p *RpcPublisher) Publish(ctx context.Context, m Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	event := new(thirdparty.MetricSnapshotChanged)
	if err := proto.Unmarshal(m.Payload, event); err != nil {
		return fmt.Errorf("decode metric snapshot event %d: %w", m.Id, err)
	}
	_, err := p.ops.ApplyMetricEvent(ctx, &ops.ApplyMetricEventRequest{
		EventId:           event.EventId,
		TenantId:          uint64(event.TenantId),
		ProviderCode:      event.ProviderCode,
		ExternalAccountId: event.ExternalAccountId,
		ExternalObjectId:  event.ExternalObjectId,
		StandardMetric:    ops.StandardMetric(event.StandardMetric),
		BucketStart:       event.BucketStart,
		BucketSeconds:     event.BucketSeconds,
		DimensionKey:      event.DimensionKey,
		Value:             event.Value,
		Currency:          event.Currency,
		SourceRevision:    event.SourceRevision,
	})
	return err
}
