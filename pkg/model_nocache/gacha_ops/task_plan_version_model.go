package gacha_ops

import (
	"gorm.io/gorm"
)

var _ TaskPlanVersionModel = (*customTaskPlanVersionModel)(nil)

type (
	// TaskPlanVersionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTaskPlanVersionModel.
	TaskPlanVersionModel interface {
		taskPlanVersionModel
		customTaskPlanVersionLogicModel
	}

	customTaskPlanVersionModel struct {
		*defaultTaskPlanVersionModel
	}

	customTaskPlanVersionLogicModel interface {
	}
)

// NewTaskPlanVersionModel returns a model for the database table.
func NewTaskPlanVersionModel(conn *gorm.DB) TaskPlanVersionModel {
	return &customTaskPlanVersionModel{
		defaultTaskPlanVersionModel: newTaskPlanVersionModel(conn),
	}
}
