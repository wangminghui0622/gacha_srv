// Package phonecrypto 手机号盲索引加密（照抄 server_v2 identity/phonecrypto）：
// AES-256-GCM 密文 + HMAC-SHA256 exact/4-gram 索引 token，AAD 绑定 tenant+user，
// encKeyID/indexVersion 支持密钥轮换；exact_token 唯一索引仲裁并发绑定。
package phonecrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const GramSize = 4

var ErrInvalidPhone = errors.New("invalid phone number or search fragment")

type Record struct {
	Ciphertext   []byte
	Nonce        []byte
	ExactToken   [32]byte
	GramTokens   [][32]byte
	EncKeyID     string
	IndexVersion int64
}

type Codec struct {
	aead         cipher.AEAD
	indexKey     [32]byte
	encKeyID     string
	indexVersion int64
}

func New(encKey, indexKey []byte, encKeyID string, indexVersion int64) (*Codec, error) {
	if len(encKey) != 32 || len(indexKey) != 32 || encKeyID == "" || len(encKeyID) > 64 || indexVersion == 0 || hmac.Equal(encKey, indexKey) {
		return nil, errors.New("distinct 32-byte phone encryption and index keys, key ID and index version required")
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	c := &Codec{aead: aead, encKeyID: encKeyID, indexVersion: indexVersion}
	copy(c.indexKey[:], indexKey)
	return c, nil
}

// NormalizeCNMobile accepts a mainland 11-digit mobile, optionally prefixed
// with +86. Canonical ciphertext stores only the 11-digit national number.
func NormalizeCNMobile(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	phone = strings.TrimPrefix(phone, "+86")
	if len(phone) != 11 || phone[0] != '1' || phone[1] < '3' || phone[1] > '9' || !digits(phone) {
		return "", ErrInvalidPhone
	}
	return phone, nil
}

// Fragment 搜索片段校验：至少 4 位、至多 11 位纯数字
func Fragment(fragment string) (string, error) {
	if len(fragment) < GramSize || len(fragment) > 11 || !digits(fragment) {
		return "", ErrInvalidPhone
	}
	return fragment, nil
}

func digits(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func (c *Codec) Seal(tenantID, userID int64, input string) (Record, error) {
	phone, err := NormalizeCNMobile(input)
	if err != nil || tenantID == 0 || userID == 0 {
		return Record{}, ErrInvalidPhone
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return Record{}, fmt.Errorf("phone nonce: %w", err)
	}
	ciphertext := c.aead.Seal(nil, nonce, []byte(phone), c.aad(tenantID, userID))
	return Record{Ciphertext: ciphertext, Nonce: nonce, ExactToken: c.token("exact", tenantID, phone), GramTokens: c.grams(tenantID, phone), EncKeyID: c.encKeyID, IndexVersion: c.indexVersion}, nil
}

func (c *Codec) Open(tenantID, userID int64, record Record) (string, error) {
	if tenantID == 0 || userID == 0 || record.EncKeyID != c.encKeyID || len(record.Nonce) != c.aead.NonceSize() {
		return "", ErrInvalidPhone
	}
	plain, err := c.aead.Open(nil, record.Nonce, record.Ciphertext, c.aad(tenantID, userID))
	if err != nil {
		return "", fmt.Errorf("decrypt phone: %w", err)
	}
	return NormalizeCNMobile(string(plain))
}

func (c *Codec) Exact(tenantID int64, input string) ([32]byte, error) {
	phone, err := NormalizeCNMobile(input)
	if err != nil || tenantID == 0 {
		return [32]byte{}, ErrInvalidPhone
	}
	return c.token("exact", tenantID, phone), nil
}

func (c *Codec) SearchTokens(tenantID int64, fragment string) ([][32]byte, error) {
	fragment, err := Fragment(fragment)
	if err != nil || tenantID == 0 {
		return nil, ErrInvalidPhone
	}
	return c.grams(tenantID, fragment), nil
}

func (c *Codec) IndexVersion() int64 { return c.indexVersion }

func (c *Codec) grams(tenantID int64, value string) [][32]byte {
	seen := make(map[[32]byte]struct{}, len(value)-GramSize+1)
	out := make([][32]byte, 0, len(value)-GramSize+1)
	for i := 0; i+GramSize <= len(value); i++ {
		token := c.token("gram4", tenantID, value[i:i+GramSize])
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func (c *Codec) token(purpose string, tenantID int64, value string) [32]byte {
	mac := hmac.New(sha256.New, c.indexKey[:])
	mac.Write([]byte("luckybox.phone." + purpose + ".v1\x00"))
	var header [10]byte
	binary.BigEndian.PutUint64(header[:8], uint64(tenantID))
	binary.BigEndian.PutUint16(header[8:], uint16(c.indexVersion))
	mac.Write(header[:])
	mac.Write([]byte(value))
	var token [32]byte
	copy(token[:], mac.Sum(nil))
	return token
}

func (c *Codec) aad(tenantID, userID int64) []byte {
	aad := make([]byte, 0, 16+16+len(c.encKeyID))
	aad = append(aad, []byte("luckybox.phone.v1\x00")...)
	var ids [16]byte
	binary.BigEndian.PutUint64(ids[:8], uint64(tenantID))
	binary.BigEndian.PutUint64(ids[8:], uint64(userID))
	aad = append(aad, ids[:]...)
	aad = append(aad, c.encKeyID...)
	return aad
}
