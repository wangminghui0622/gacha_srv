// totp e2e 测试工具：RFC 6238（SHA1/6位/30s），纯标准库实现，
// 与 app_admin/rpc/internal/core/totp.go 参数一致（场景 24 管理控制台两步登录用）。
// 用法：go run test/tools/totp/main.go <Base32Secret>
package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "" {
		fmt.Fprintln(os.Stderr, "用法: totp <Base32Secret>")
		os.Exit(1)
	}
	secret := strings.ToUpper(strings.ReplaceAll(os.Args[1], " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		fmt.Fprintln(os.Stderr, "base32 解码失败:", err)
		os.Exit(1)
	}
	counter := time.Now().Unix() / 30
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(counter))
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0xf
	code := (binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff) % 1000000
	fmt.Printf("%06d", code)
}
