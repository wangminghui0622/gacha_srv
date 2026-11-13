// Package productcode 商品编码生成：P + 商品 ID 的可逆混淆字符串。
package productcode

import (
	"fmt"

	"gacha_srv/pkg/utils/numcrypt"
)

const (
	prefix = "P"
	// 与 test/tools/id/main.go 中使用的开发密钥保持一致。
	key = "0123456789abcdef0123456789abcdef"
)

var cipher = mustCipher()

func mustCipher() *numcrypt.Cipher {
	c, err := numcrypt.New([]byte(key))
	if err != nil {
		panic("productcode: invalid cipher key: " + err.Error())
	}
	return c
}

// Generate 根据商品 ID 生成 product_code。
func Generate(productID uint64) (string, error) {
	encrypted, err := cipher.Encrypt(productID)
	if err != nil {
		return "", err
	}
	return prefix + encrypted, nil
}

// Decode 解析 product_code 中的商品 ID。
func Decode(code string) (uint64, error) {
	if len(code) < 2 || code[:1] != prefix {
		return 0, fmt.Errorf("productcode: code must start with %q", prefix)
	}
	return cipher.Decrypt(code[1:])
}
