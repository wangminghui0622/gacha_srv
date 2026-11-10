package core

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2idHasher 控制台账号口令哈希（参数与 server_v2 identity 一致：
// 19MB 内存 / 2 次迭代 / 单线程 / 16 字节盐 / 32 字节密钥）。
type Argon2idHasher struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2idHasher 默认参数。
func DefaultArgon2idHasher() Argon2idHasher {
	return Argon2idHasher{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}
}

// Hash 生成 `$argon2id$v=19$m=...,t=...,p=...$salt$hash` 格式（口令至少 12 字节）。
func (h Argon2idHasher) Hash(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("密码至少 12 字节")
	}
	salt := make([]byte, h.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(password), salt, h.Iterations, h.Memory, h.Parallelism, h.KeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", h.Memory, h.Iterations, h.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum)), nil
}

// Verify 校验口令与哈希是否匹配（常数时间比较）。
func (h Argon2idHasher) Verify(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations, parallelism uint64
	for _, entry := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(entry, "=", 2)
		if len(kv) != 2 {
			return false
		}
		value, err := strconv.ParseUint(kv[1], 10, 32)
		if err != nil {
			return false
		}
		switch kv[0] {
		case "m":
			memory = value
		case "t":
			iterations = value
		case "p":
			parallelism = value
		}
	}
	if memory == 0 || iterations == 0 || parallelism == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
