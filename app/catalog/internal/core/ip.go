package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	IPNameMaxLength   = 128
	IPSortWeightLimit = 1_000_000
)

// BuildIP 校验并归一化 IP 名称/排序权重。
func BuildIP(name string, sortWeight int32) (string, int32, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", 0, fmt.Errorf("IP 名称必填")
	}
	if utf8.RuneCountInString(name) > IPNameMaxLength {
		return "", 0, fmt.Errorf("IP 名称不能超过 %d 个字符", IPNameMaxLength)
	}
	if strings.ContainsAny(name, "\r\n\t") {
		return "", 0, fmt.Errorf("IP 名称不能包含换行或制表符")
	}
	if sortWeight < 0 || sortWeight > IPSortWeightLimit {
		return "", 0, fmt.Errorf("排序权重必须在 0-%d 之间", IPSortWeightLimit)
	}
	return name, sortWeight, nil
}
