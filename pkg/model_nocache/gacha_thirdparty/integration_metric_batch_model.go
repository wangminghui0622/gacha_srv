package gacha_thirdparty

import (
	"context"

	"github.com/SpectatorNan/gorm-zero/gormc"
	"gorm.io/gorm"
)

var _ IntegrationMetricBatchModel = (*customIntegrationMetricBatchModel)(nil)

type (
	// IntegrationMetricBatchModel is an interface to be customized, add more methods here,
	// and implement the added methods in customIntegrationMetricBatchModel.
	IntegrationMetricBatchModel interface {
		integrationMetricBatchModel
		customIntegrationMetricBatchLogicModel
	}

	customIntegrationMetricBatchModel struct {
		*defaultIntegrationMetricBatchModel
	}

	customIntegrationMetricBatchLogicModel interface {
		// 幂等键查询（server_v2 GetBatchByRequest / GetBatchByKey 同款，非唯一索引手写）。
		FindOneByTenantIdRequestId(ctx context.Context, tenantId uint64, requestId string) (*IntegrationMetricBatch, error)
		FindOneByTenantIdProviderCodeIdempotencyKey(ctx context.Context, tenantId uint64, providerCode string, idempotencyKey string) (*IntegrationMetricBatch, error)
	}
)

// NewIntegrationMetricBatchModel returns a model for the database table.
func NewIntegrationMetricBatchModel(conn *gorm.DB) IntegrationMetricBatchModel {
	return &customIntegrationMetricBatchModel{
		defaultIntegrationMetricBatchModel: newIntegrationMetricBatchModel(conn),
	}
}

// FindOneByTenantIdRequestId 按 (tenant_id, request_id) 查批次（幂等键一）。
func (m *customIntegrationMetricBatchModel) FindOneByTenantIdRequestId(ctx context.Context, tenantId uint64, requestId string) (*IntegrationMetricBatch, error) {
	return m.findOneBy(ctx, "`tenant_id` = ? AND `request_id` = ?", tenantId, requestId)
}

// FindOneByTenantIdProviderCodeIdempotencyKey 按 (tenant_id, provider_code, idempotency_key) 查批次（幂等键二）。
func (m *customIntegrationMetricBatchModel) FindOneByTenantIdProviderCodeIdempotencyKey(ctx context.Context, tenantId uint64, providerCode string, idempotencyKey string) (*IntegrationMetricBatch, error) {
	return m.findOneBy(ctx, "`tenant_id` = ? AND `provider_code` = ? AND `idempotency_key` = ?", tenantId, providerCode, idempotencyKey)
}

func (m *customIntegrationMetricBatchModel) findOneBy(ctx context.Context, where string, args ...any) (*IntegrationMetricBatch, error) {
	var resp IntegrationMetricBatch
	err := m.conn.WithContext(ctx).Model(&IntegrationMetricBatch{}).
		Where(where, args...).Take(&resp).Error
	switch err {
	case nil:
		return &resp, nil
	case gormc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
