package gacha_ops

import (
	"gorm.io/gorm"
)

var _ TaskPlanHeadModel = (*customTaskPlanHeadModel)(nil)

type (
	// TaskPlanHeadModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTaskPlanHeadModel.
	TaskPlanHeadModel interface {
		taskPlanHeadModel
		customTaskPlanHeadLogicModel
	}

	customTaskPlanHeadModel struct {
		*defaultTaskPlanHeadModel
	}

	customTaskPlanHeadLogicModel interface {
	}
)

// NewTaskPlanHeadModel returns a model for the database table.
func NewTaskPlanHeadModel(conn *gorm.DB) TaskPlanHeadModel {
	return &customTaskPlanHeadModel{
		defaultTaskPlanHeadModel: newTaskPlanHeadModel(conn),
	}
}
