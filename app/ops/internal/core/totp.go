// Package core 运营服务纯逻辑：口令哈希 / TOTP / 控制台会话 / 授权决策。
package core

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP 参数（RFC 6238，与 server_v2 pkg/totp 一致：SHA1/6 位/30 秒/±1 窗口）。
const (
	totpDefaultSecretLength = 20
	totpDefaultStepSeconds  = 30
	totpDefaultDigits       = 6
)

// GenerateTotpSecret 生成 base32（无填充）密钥。
func GenerateTotpSecret() (string, error) {
	buf := make([]byte, totpDefaultSecretLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(buf), "="), nil
}

// ValidateTotpCode 校验 6 位验证码（±1 时间窗口）。
func ValidateTotpCode(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDefaultDigits {
		return false
	}
	for _, offset := range []int64{-1, 0, 1} {
		if GenerateTotpCode(secret, now.Add(time.Duration(offset*totpDefaultStepSeconds)*time.Second)) == code {
			return true
		}
	}
	return false
}

// GenerateTotpCode 按时间戳生成 6 位验证码。
func GenerateTotpCode(secret string, t time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return ""
	}
	counter := uint64(t.Unix() / totpDefaultStepSeconds)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg[:])
	hash := mac.Sum(nil)
	offset := hash[len(hash)-1] & 0x0f
	binaryCode := (int(hash[offset])&0x7f)<<24 |
		(int(hash[offset+1])&0xff)<<16 |
		(int(hash[offset+2])&0xff)<<8 |
		(int(hash[offset+3]) & 0xff)
	return fmt.Sprintf("%06d", binaryCode%1000000)
}

// BuildTotpURI 生成 otpauth:// 绑定 URI（前端二维码用）。
func BuildTotpURI(issuer, account, secret string) string {
	issuer = strings.TrimSpace(issuer)
	account = strings.TrimSpace(account)
	if issuer == "" {
		issuer = "Gacha Console"
	}
	if account == "" {
		account = "operator"
	}
	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, account))
	values := url.Values{}
	values.Set("secret", secret)
	values.Set("issuer", issuer)
	values.Set("algorithm", "SHA1")
	values.Set("digits", fmt.Sprint(totpDefaultDigits))
	values.Set("period", fmt.Sprint(totpDefaultStepSeconds))
	return fmt.Sprintf("otpauth://totp/%s?%s", label, values.Encode())
}
