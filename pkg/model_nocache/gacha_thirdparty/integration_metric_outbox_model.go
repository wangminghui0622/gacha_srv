package gacha_thirdparty

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var _ IntegrationMetricOutboxModel = (*customIntegrationMetricOutboxModel)(nil)

type (
	// IntegrationMetricOutboxModel is an interface to be customized, add more methods here,
	// and implement the added methods in customIntegrationMetricOutboxModel.
	IntegrationMetricOutboxModel interface {
		integrationMetricOutboxModel
		customIntegrationMetricOutboxLogicModel
	}

	customIntegrationMetricOutboxModel struct {
		*defaultIntegrationMetricOutboxModel
	}

	// OutboxMessage 待投递事件（server_v2 shared.Message 同款形状；Attempts 为本次 claim 后的次数）。
	OutboxMessage struct {
		Id          uint64
		AggregateId string
		EventType   string
		Payload     []byte
		Attempts    uint64
	}

	customIntegrationMetricOutboxLogicModel interface {
		// 语义照抄 server_v2 outboxStore（integration internal/data/outbox.go）：
		//   ClaimBatch：FOR UPDATE SKIP LOCKED 抢最早可用行（含租约过期的 status=2），
		//     同一 aggregate 前序事件未发完（status<>3）不投递；抢中行标记 status=2 + 租约。
		//   MarkPublished：status=2 且租约归我才可置 3（租约丢失报错）。
		//   Reschedule：status=2 且租约归我才可退回 1（available_at 退避重试）。
		ClaimBatch(ctx context.Context, now time.Time, batch int, lease time.Duration, owner string) ([]OutboxMessage, error)
		MarkPublished(ctx context.Context, id uint64, owner string, publishedAt time.Time) error
		Reschedule(ctx context.Context, id uint64, owner string, availableAt time.Time, lastError string) error
	}
)

// NewIntegrationMetricOutboxModel returns a model for the database table.
func NewIntegrationMetricOutboxModel(conn *gorm.DB) IntegrationMetricOutboxModel {
	return &customIntegrationMetricOutboxModel{
		defaultIntegrationMetricOutboxModel: newIntegrationMetricOutboxModel(conn),
	}
}

// ClaimBatch 抢批：SQL 与 server_v2 LockMetricOutboxBatch + MarkMetricOutboxProcessing 逐字同款。
func (m *customIntegrationMetricOutboxModel) ClaimBatch(ctx context.Context, now time.Time, batch int, lease time.Duration, owner string) ([]OutboxMessage, error) {
	until := now.Add(lease)
	out := make([]OutboxMessage, 0, batch)
	err := m.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			Id          uint64
			AggregateId string
			EventType   string
			Payload     string
			Attempts    uint64
		}
		err := tx.Raw(
			"SELECT o.id,o.aggregate_id,o.event_type,o.payload,o.attempts FROM `integration_metric_outbox` o "+
				"WHERE ((o.status=1 AND o.available_at<=?) OR (o.status=2 AND o.locked_until<?)) "+
				"AND NOT EXISTS (SELECT 1 FROM `integration_metric_outbox` earlier WHERE earlier.aggregate_id=o.aggregate_id AND earlier.id<o.id AND earlier.status<>3) "+
				"ORDER BY o.id LIMIT ? FOR UPDATE SKIP LOCKED", now, now, batch).Scan(&rows).Error
		if err != nil {
			return err
		}
		for _, x := range rows {
			res := tx.Exec(
				"UPDATE `integration_metric_outbox` SET `status`=2,`lease_owner`=?,`locked_until`=?,`attempts`=`attempts`+1,`last_error`='' "+
					"WHERE `id`=? AND ((`status`=1 AND `available_at`<=?) OR (`status`=2 AND `locked_until`<?))",
				owner, until, x.Id, now, now)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 1 {
				out = append(out, OutboxMessage{Id: x.Id, AggregateId: x.AggregateId, EventType: x.EventType, Payload: []byte(x.Payload), Attempts: x.Attempts + 1})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MarkPublished 投递成功：置 3（server_v2 MarkMetricOutboxPublished 同款，租约丢失报错）。
func (m *customIntegrationMetricOutboxModel) MarkPublished(ctx context.Context, id uint64, owner string, publishedAt time.Time) error {
	res := m.conn.WithContext(ctx).Exec(
		"UPDATE `integration_metric_outbox` SET `status`=3,`published_at`=?,`lease_owner`='',`locked_until`=NULL,`last_error`='' "+
			"WHERE `id`=? AND `status`=2 AND `lease_owner`=?", publishedAt, id, owner)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("metric outbox lease lost before ack")
	}
	return nil
}

// Reschedule 投递失败：退回待发（server_v2 RescheduleMetricOutbox 同款，租约丢失报错）。
func (m *customIntegrationMetricOutboxModel) Reschedule(ctx context.Context, id uint64, owner string, availableAt time.Time, lastError string) error {
	res := m.conn.WithContext(ctx).Exec(
		"UPDATE `integration_metric_outbox` SET `status`=1,`available_at`=?,`lease_owner`='',`locked_until`=NULL,`last_error`=? "+
			"WHERE `id`=? AND `status`=2 AND `lease_owner`=?", availableAt, lastError, id, owner)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("metric outbox lease lost before nack")
	}
	return nil
}
