package gacha_marketing

import (
	"gorm.io/gorm"
)

var _ QuoteLocksModel = (*customQuoteLocksModel)(nil)

type (
	// QuoteLocksModel is an interface to be customized, add more methods here,
	// and implement the added methods in customQuoteLocksModel.
	QuoteLocksModel interface {
		quoteLocksModel
		customQuoteLocksLogicModel
	}

	customQuoteLocksModel struct {
		*defaultQuoteLocksModel
	}

	customQuoteLocksLogicModel interface {
	}
)

// NewQuoteLocksModel returns a model for the database table.
func NewQuoteLocksModel(conn *gorm.DB) QuoteLocksModel {
	return &customQuoteLocksModel{
		defaultQuoteLocksModel: newQuoteLocksModel(conn),
	}
}
