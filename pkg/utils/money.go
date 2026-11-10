// Package utils 通用工具
package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// yuanRe 金额字符串格式：正数、最多两位小数（"10" / "10.5" / "10.50"）
var yuanRe = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)

// ErrMoneyFormat 金额格式错误
var ErrMoneyFormat = fmt.Errorf("金额格式无效（正数、最多两位小数）")

// YuanToFen 元字符串（"12.34"）转分（1234）。字符串解析，不经 float，避免精度丢失。
func YuanToFen(s string) (int64, error) {
	if !yuanRe.MatchString(s) {
		return 0, ErrMoneyFormat
	}
	i := strings.IndexByte(s, '.')
	var yuan, frac string
	if i < 0 {
		yuan, frac = s, ""
	} else {
		yuan, frac = s[:i], s[i+1:]
	}
	y, err := strconv.ParseInt(yuan, 10, 64)
	if err != nil {
		return 0, ErrMoneyFormat
	}
	var f int64
	switch len(frac) {
	case 1:
		f, _ = strconv.ParseInt(frac+"0", 10, 64)
	case 2:
		f, _ = strconv.ParseInt(frac, 10, 64)
	}
	return y*100 + f, nil
}

// FenToYuan 分转元字符串（1234 → "12.34"；负数带符号）。
func FenToYuan(fen int64) string {
	sign := ""
	if fen < 0 {
		sign = "-"
		fen = -fen
	}
	return fmt.Sprintf("%s%d.%02d", sign, fen/100, fen%100)
}
