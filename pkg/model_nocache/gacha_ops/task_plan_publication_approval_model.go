package gacha_ops

import (
	"gorm.io/gorm"
)

var _ TaskPlanPublicationApprovalModel = (*customTaskPlanPublicationApprovalModel)(nil)

type (
	// TaskPlanPublicationApprovalModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTaskPlanPublicationApprovalModel.
	TaskPlanPublicationApprovalModel interface {
		taskPlanPublicationApprovalModel
		customTaskPlanPublicationApprovalLogicModel
	}

	customTaskPlanPublicationApprovalModel struct {
		*defaultTaskPlanPublicationApprovalModel
	}

	customTaskPlanPublicationApprovalLogicModel interface {
	}
)

// NewTaskPlanPublicationApprovalModel returns a model for the database table.
func NewTaskPlanPublicationApprovalModel(conn *gorm.DB) TaskPlanPublicationApprovalModel {
	return &customTaskPlanPublicationApprovalModel{
		defaultTaskPlanPublicationApprovalModel: newTaskPlanPublicationApprovalModel(conn),
	}
}
