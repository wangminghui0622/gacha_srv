// Package numcrypt 数字与 Base62 字符串的可逆混淆（Feistel + HMAC-SHA256）。
// 算法与 test/tools/id/main.go 保持一致。
package numcrypt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	half     = uint64(100000000) // 10^8
	domain   = half * half       // 10^16
	rounds   = 8
	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var decodeMap [256]int

func init() {
	for i := range decodeMap {
		decodeMap[i] = -1
	}
	for i, c := range alphabet {
		decodeMap[c] = i
	}
}

// Cipher 持有密钥，可复用；并发安全（无内部状态）。
type Cipher struct {
	key []byte
}

// New 创建 Cipher，key 至少 16 字节。
func New(key []byte) (*Cipher, error) {
	if len(key) < 16 {
		return nil, errors.New("numcrypt: key must be at least 16 bytes")
	}
	k := make([]byte, len(key))
	copy(k, key)
	return &Cipher{key: k}, nil
}

// Encrypt 数字 -> 字符串。n 必须 < 10^16。
func (c *Cipher) Encrypt(n uint64) (string, error) {
	if n >= domain {
		return "", fmt.Errorf("numcrypt: number out of range: %d (max %d)", n, domain-1)
	}
	enc := feistelEncrypt(n, c.key)
	return base62Encode(enc), nil
}

// Decrypt 字符串 -> 数字。非法字符、溢出、越域均返回错误。
func (c *Cipher) Decrypt(s string) (uint64, error) {
	enc, err := base62Decode(s)
	if err != nil {
		return 0, err
	}
	if enc >= domain {
		return 0, fmt.Errorf("numcrypt: decoded value out of domain: %d", enc)
	}
	return feistelDecrypt(enc, c.key), nil
}

func roundFn(r uint64, key []byte, round int) uint64 {
	var buf [12]byte
	binary.BigEndian.PutUint64(buf[:8], r)
	binary.BigEndian.PutUint32(buf[8:], uint32(round))

	mac := hmac.New(sha256.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8]) % half
}

func feistelEncrypt(n uint64, key []byte) uint64 {
	l, r := n/half, n%half
	for i := 0; i < rounds; i++ {
		l, r = r, (l+roundFn(r, key, i))%half
	}
	return l*half + r
}

func feistelDecrypt(n uint64, key []byte) uint64 {
	l, r := n/half, n%half
	for i := rounds - 1; i >= 0; i-- {
		l, r = (r+half-roundFn(l, key, i))%half, l
	}
	return l*half + r
}

func base62Encode(n uint64) string {
	if n == 0 {
		return string(alphabet[0])
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = alphabet[n%62]
		n /= 62
	}
	return string(buf[i:])
}

func base62Decode(s string) (uint64, error) {
	if len(s) == 0 {
		return 0, errors.New("numcrypt: empty string")
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		v := decodeMap[s[i]]
		if v < 0 {
			return 0, fmt.Errorf("numcrypt: invalid base62 char: %q", s[i])
		}
		if n > (^uint64(0)-uint64(v))/62 {
			return 0, errors.New("numcrypt: overflow")
		}
		n = n*62 + uint64(v)
	}
	return n, nil
}
