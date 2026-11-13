package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	SeriesNameMaxLength   = 128
	SeriesSortWeightLimit = 1_000_000
)

// BuildSeries 校验并归一化系列名称/品牌/排序权重。
func BuildSeries(name string, brandID uint64, sortWeight int32) (string, uint64, int32, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", 0, 0, fmt.Errorf("系列名称必填")
	}
	if utf8.RuneCountInString(name) > SeriesNameMaxLength {
		return "", 0, 0, fmt.Errorf("系列名称不能超过 %d 个字符", SeriesNameMaxLength)
	}
	if strings.ContainsAny(name, "\r\n\t") {
		return "", 0, 0, fmt.Errorf("系列名称不能包含换行或制表符")
	}
	if brandID == 0 {
		return "", 0, 0, fmt.Errorf("所属品牌必填")
	}
	if sortWeight < 0 || sortWeight > SeriesSortWeightLimit {
		return "", 0, 0, fmt.Errorf("排序权重必须在 0-%d 之间", SeriesSortWeightLimit)
	}
	return name, brandID, sortWeight, nil
}
