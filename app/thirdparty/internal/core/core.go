// Package core 纯函数：手机号归一化 / 验证码生成（照抄 server_v2 integration/sms）
package core

import (
	"crypto/rand"
	"errors"
	"math/big"
	"regexp"
	"strings"
)

var cnMobile = regexp.MustCompile(`^1[3-9]\d{9}$`)

// ErrInvalidCNMobile 手机号格式非法（非大陆 11 位 1[3-9] 开头）
var ErrInvalidCNMobile = errors.New("invalid mainland mobile")

// NormalizeCNMobile 剥 +86/首尾空白并校验大陆手机号格式
func NormalizeCNMobile(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	phone = strings.TrimPrefix(phone, "+86")
	phone = strings.TrimSpace(phone)
	if !cnMobile.MatchString(phone) {
		return "", ErrInvalidCNMobile
	}
	return phone, nil
}

// RandomDigits 生成 size 位随机数字串（crypto/rand，验证码用）
func RandomDigits(size int) (string, error) {
	out := make([]byte, size)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		out[i] = byte('0' + n.Int64())
	}
	return string(out), nil
}
