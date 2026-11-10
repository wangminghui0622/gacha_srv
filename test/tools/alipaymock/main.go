// e2e 工具：支付宝 openapi mock（场景 32 用，thirdparty-rpc yaml Alipay.APIBase
// 指向 http://127.0.0.1:9411）。
// 契约：
//   - POST 表单 method=alipay.system.oauth.token：
//     code=err → error_response isv.code-invalid（→ thirdparty 报 9008）；
//     grant_type=authorization_code → 回 alipay_system_oauth_token_response
//     （user_id/access_token/refresh_token/open_id 均含 code 指纹）；
//     grant_type=refresh_token 同款；其他 method/grant → error_response。
//   - 请求验签：本目录 dev_public.pem 存在时（dev 密钥对公钥），对待签内容
//     （除 sign 外全部表单参数 key 升序 k=v& 连接）做 SHA256withRSA 验签，
//     失败 → error_response isv.invalid-signature（端到端证明客户端签名正确）。
// 端口占用时自动退出。
package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	addr := "127.0.0.1:9411"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("alipaymock listen %s failed: %v\n", addr, err)
		os.Exit(1)
	}
	verifier := loadPublicKey("dev_public.pem")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if err := checkSign(r, verifier); err != nil {
			writeJSON(w, map[string]any{"error_response": map[string]any{
				"code": "40003", "msg": "isv.invalid-signature", "sub_code": "isv.invalid-signature", "sub_msg": err.Error(),
			}})
			return
		}
		switch r.Form.Get("method") {
		case "alipay.system.oauth.token":
			writeJSON(w, oauthTokenReply(r))
		default:
			writeJSON(w, map[string]any{"error_response": map[string]any{
				"code": "40001", "msg": "isv.unsupported-method", "sub_code": "isv.unsupported-method",
				"sub_msg": "method not supported: " + r.Form.Get("method"),
			}})
		}
	})
	fmt.Printf("alipaymock listening on %s (verify=%v)\n", addr, verifier != nil)
	if err := http.Serve(ln, nil); err != nil {
		fmt.Printf("alipaymock stopped: %v\n", err)
		os.Exit(1)
	}
}

// oauthTokenReply alipay.system.oauth.token 应答（字段含 code/token 指纹便于断言）。
func oauthTokenReply(r *http.Request) map[string]any {
	errNode := func(code, msg string) map[string]any {
		return map[string]any{"error_response": map[string]any{
			"code": code, "msg": msg, "sub_code": code, "sub_msg": msg,
		}}
	}
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		code := strings.TrimSpace(r.Form.Get("code"))
		if code == "" || code == "err" {
			return errNode("40002", "isv.code-invalid")
		}
		return tokenNode(code, "auth")
	case "refresh_token":
		token := strings.TrimSpace(r.Form.Get("refresh_token"))
		if token == "" || token == "err" {
			return errNode("40002", "isv.refresh-token-invalid")
		}
		return tokenNode(token, "refresh")
	default:
		return errNode("40002", "isv.grant-type-invalid")
	}
}

func tokenNode(fingerprint, kind string) map[string]any {
	return map[string]any{"alipay_system_oauth_token_response": map[string]any{
		"user_id":       "dev-alipay-user-" + fingerprint,
		"access_token":  "dev-alipay-at-" + kind + "-" + fingerprint,
		"refresh_token": "dev-alipay-rt-" + kind + "-" + fingerprint,
		"open_id":       "dev-alipay-open-" + kind + "-" + fingerprint,
	}}
}

// checkSign 请求验签（公钥未配置跳过）；待签内容=除 sign 外全部表单参数升序 k=v& 连接。
func checkSign(r *http.Request, verifier *rsa.PublicKey) error {
	if verifier == nil {
		return nil
	}
	sign := r.Form.Get("sign")
	if sign == "" {
		return fmt.Errorf("sign missing")
	}
	keys := make([]string, 0, len(r.Form))
	for k := range r.Form {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+r.Form.Get(k))
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "&")))
	sig, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(verifier, crypto.SHA256, digest[:], sig)
}

// loadPublicKey 从当前目录或工具目录读取 dev 公钥 PEM（缺失/非法 → nil 跳过验签）。
func loadPublicKey(name string) *rsa.PublicKey {
	var candidates []string
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, name))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), name))
	}
	for _, path := range candidates {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		block, _ := pem.Decode(raw)
		if block == nil {
			continue
		}
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			continue
		}
		pub, _ := key.(*rsa.PublicKey)
		return pub
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
