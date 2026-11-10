package gacha_ops

import (
	"gorm.io/gorm"
)

var _ OpsAuditLogsModel = (*customOpsAuditLogsModel)(nil)

type (
	// OpsAuditLogsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOpsAuditLogsModel.
	OpsAuditLogsModel interface {
		opsAuditLogsModel
		customOpsAuditLogsLogicModel
	}

	customOpsAuditLogsModel struct {
		*defaultOpsAuditLogsModel
	}

	customOpsAuditLogsLogicModel interface {
	}
)

// NewOpsAuditLogsModel returns a model for the database table.
func NewOpsAuditLogsModel(conn *gorm.DB) OpsAuditLogsModel {
	return &customOpsAuditLogsModel{
		defaultOpsAuditLogsModel: newOpsAuditLogsModel(conn),
	}
}
