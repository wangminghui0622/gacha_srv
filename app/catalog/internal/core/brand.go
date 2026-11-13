package core

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	BrandNameMaxLength = 128
	BrandLogoMaxLength = 512
)

// BuildBrand 校验并归一化品牌名称/Logo 地址。
func BuildBrand(name, logo string) (string, string, error) {
	name = strings.TrimSpace(name)
	logo = strings.TrimSpace(logo)
	if name == "" {
		return "", "", fmt.Errorf("品牌名称必填")
	}
	if utf8.RuneCountInString(name) > BrandNameMaxLength {
		return "", "", fmt.Errorf("品牌名称不能超过 %d 个字符", BrandNameMaxLength)
	}
	if strings.ContainsAny(name, "\r\n\t") {
		return "", "", fmt.Errorf("品牌名称不能包含换行或制表符")
	}
	if len(logo) > BrandLogoMaxLength {
		return "", "", fmt.Errorf("Logo 地址不能超过 %d 个字符", BrandLogoMaxLength)
	}
	return name, logo, nil
}
