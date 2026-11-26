// Package ordercode 抽盒订单对外编码：o + Base62(Feistel/HMAC 混淆)。
// 内部订单号 = YYYYMMDD * 10^9 + 序号；对外只编码序号部分，日期前缀不外露。
package ordercode

import (
	"fmt"
	"strings"

	"gacha_srv/pkg/utils/numcrypt"
)

const (
	Prefix = "o"
	// key 与 test/tools/id、productcode、publicid 使用的开发密钥保持一致。
	key = "0123456789abcdef0123456789abcdef"
)

var cipher = mustCipher()

func mustCipher() *numcrypt.Cipher {
	c, err := numcrypt.New([]byte(key))
	if err != nil {
		panic("ordercode: invalid cipher key: " + err.Error())
	}
	return c
}

// Encode 序号 → 对外订单号（o 前缀）。
func Encode(seq uint64) (string, error) {
	if seq == 0 {
		return "", fmt.Errorf("ordercode: invalid sequence: 0")
	}
	encoded, err := cipher.Encrypt(seq)
	if err != nil {
		return "", err
	}
	return Prefix + encoded, nil
}

// Decode 对外订单号 → 序号。
func Decode(code string) (uint64, error) {
	code = strings.TrimSpace(code)
	if !strings.HasPrefix(code, Prefix) || len(code) == len(Prefix) {
		return 0, fmt.Errorf("ordercode: code must start with %q", Prefix)
	}
	return cipher.Decrypt(code[len(Prefix):])
}
