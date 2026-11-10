// Package outbox 指标事件投递调度器（M15d D2：无 Kafka，单实例轮询直投 ops-rpc）。
// 调度语义照抄 server_v2 pkg/outbox/outbox.go（Dispatcher/DefaultOptions/Backoff）：
//
//	500ms 轮询、批 100、租约 30s、指数退避 1s<<attempt 封顶 5min；
//	投递失败 → Reschedule 退回待发（available_at=now+backoff），投递成功 → Ack 置已发。
//
// 与老项目差异：Publisher 不是 Kafka，而是 ops-rpc ApplyMetricEvent RPC
// （at-least-once，ops 侧 inbox event_id 去重 + 修订号守卫保证幂等）。
package outbox

import (
	"context"
	"errors"
	"sync"
	"time"

	"gacha_srv/pkg/model_nocache/gacha_thirdparty"

	"github.com/zeromicro/go-zero/core/logx"
)

// Message 与老项目 shared.Message 同款（直接用模型层 OutboxMessage）。
type Message = gacha_thirdparty.OutboxMessage

// Store outbox 存储抽象（模型层 IntegrationMetricOutboxModel 已实现）。
type Store interface {
	ClaimBatch(ctx context.Context, now time.Time, batch int, lease time.Duration, owner string) ([]Message, error)
	MarkPublished(ctx context.Context, id uint64, owner string, publishedAt time.Time) error
	Reschedule(ctx context.Context, id uint64, owner string, availableAt time.Time, lastError string) error
}

// Publisher 事件投递目标（RpcPublisher → ops-rpc ApplyMetricEvent）。
type Publisher interface {
	Publish(ctx context.Context, m Message) error
}

// Options 调度参数（server_v2 outbox.Options 同款）。
type Options struct {
	Batch                       int32
	Interval, Lease, MaxBackoff time.Duration
}

// DefaultOptions server_v2 DefaultOptions 同款。
func DefaultOptions() Options {
	return Options{Batch: 100, Interval: 500 * time.Millisecond, Lease: 30 * time.Second, MaxBackoff: 5 * time.Minute}
}

// Dispatcher 单实例轮询调度器（server_v2 Dispatcher 移植，日志走 logx）。
type Dispatcher struct {
	store     Store
	publisher Publisher
	options   Options
	stop      chan struct{}
	once      sync.Once
}

// NewDispatcher 构建调度器。
func NewDispatcher(store Store, publisher Publisher, options Options) (*Dispatcher, error) {
	if store == nil || publisher == nil {
		return nil, errors.New("outbox store and publisher are required")
	}
	if options.Batch <= 0 || options.Interval <= 0 || options.Lease <= 0 || options.MaxBackoff <= 0 {
		return nil, errors.New("outbox options must be positive")
	}
	return &Dispatcher{store: store, publisher: publisher, options: options, stop: make(chan struct{})}, nil
}

// Run 启动轮询（server_v2 Start 同款：先跑一轮再进 ticker；ctx 取消或 Stop 退出）。
func (d *Dispatcher) Run(ctx context.Context) error {
	if err := d.DispatchOnce(ctx); err != nil {
		logx.Errorf("outbox dispatch: %v", err)
	}
	ticker := time.NewTicker(d.options.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-d.stop:
			return nil
		case <-ticker.C:
			if err := d.DispatchOnce(ctx); err != nil {
				logx.Errorf("outbox dispatch: %v", err)
			}
		}
	}
}

// Stop 停止调度（幂等）。
func (d *Dispatcher) Stop() {
	d.once.Do(func() { close(d.stop) })
}

// DispatchOnce 抢一批并逐条投递（server_v2 同款：失败截断 512 字节退避重排，成功 Ack）。
func (d *Dispatcher) DispatchOnce(ctx context.Context) error {
	messages, err := d.store.ClaimBatch(ctx, time.Now().UTC(), int(d.options.Batch), d.options.Lease, d.owner())
	if err != nil {
		return err
	}
	for _, message := range messages {
		if err = d.publisher.Publish(ctx, message); err != nil {
			lastError := err.Error()
			if len(lastError) > 512 {
				lastError = lastError[:512]
			}
			if nackErr := d.store.Reschedule(ctx, message.Id, d.owner(), time.Now().UTC().Add(Backoff(uint32(message.Attempts), d.options.MaxBackoff)), lastError); nackErr != nil {
				return nackErr
			}
			continue
		}
		if err = d.store.MarkPublished(ctx, message.Id, d.owner(), time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

// owner 单实例租约名（D2：调度器只在 thirdparty-rpc 单实例跑，固定名即可）。
func (d *Dispatcher) owner() string {
	return "thirdparty-single"
}

// Backoff server_v2 outbox.Backoff 同款：1s<<attempt（attempt 上限 30）封顶 max。
func Backoff(attempt uint32, max time.Duration) time.Duration {
	if attempt > 0 {
		attempt--
	}
	if attempt > 30 {
		attempt = 30
	}
	delay := time.Second * time.Duration(1<<attempt)
	if delay > max {
		return max
	}
	return delay
}
