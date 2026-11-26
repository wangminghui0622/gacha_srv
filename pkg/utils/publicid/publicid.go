// Package publicid 对外用户 ID 的生成与解析：u + Base62(Feistel/HMAC 混淆)。
// 数据库与内部 RPC 仍使用自增数字主键，只有对外通信使用字符串 ID。
package publicid

import (
	"fmt"
	"strconv"
	"strings"

	"gacha_srv/pkg/utils/numcrypt"
)

const (
	// UserPrefix 用户对外 ID 前缀，后续可按实体扩展（如 o=订单、p=赏池）。
	UserPrefix = "u"
	// key 与 test/tools/id、productcode 使用的开发密钥保持一致。
	key = "0123456789abcdef0123456789abcdef"
)

var cipher = mustCipher()

func mustCipher() *numcrypt.Cipher {
	c, err := numcrypt.New([]byte(key))
	if err != nil {
		panic("publicid: invalid cipher key: " + err.Error())
	}
	return c
}

// EncodeUserID 数字用户 ID → 对外字符串（u 前缀）。
func EncodeUserID(userID int64) (string, error) {
	if userID <= 0 {
		return "", fmt.Errorf("publicid: invalid user id: %d", userID)
	}
	encoded, err := cipher.Encrypt(uint64(userID))
	if err != nil {
		return "", err
	}
	return UserPrefix + encoded, nil
}

// MustEncodeUserID 同 EncodeUserID，编码失败返回空串（用于展示层兜底）。
func MustEncodeUserID(userID int64) string {
	encoded, err := EncodeUserID(userID)
	if err != nil {
		return ""
	}
	return encoded
}

// DecodeUserID 对外字符串 → 数字用户 ID；必须带 u 前缀。
func DecodeUserID(publicID string) (int64, error) {
	publicID = strings.TrimSpace(publicID)
	if !strings.HasPrefix(publicID, UserPrefix) || len(publicID) == len(UserPrefix) {
		return 0, fmt.Errorf("publicid: user id must start with %q", UserPrefix)
	}
	decoded, err := cipher.Decrypt(publicID[len(UserPrefix):])
	if err != nil {
		return 0, err
	}
	if decoded == 0 || decoded > uint64(^uint64(0)>>1) {
		return 0, fmt.Errorf("publicid: decoded user id out of range: %d", decoded)
	}
	return int64(decoded), nil
}

// ParseUserID 兼容后台输入：纯数字按数字 ID 解析，u 开头按对外 ID 解析。
func ParseUserID(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("publicid: empty user id")
	}
	if strings.HasPrefix(value, UserPrefix) {
		return DecodeUserID(value)
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("publicid: invalid user id: %q", value)
	}
	return n, nil
}
