package alipay

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"gacha_srv/app/thirdparty/thirdparty"
)

// genKey 生成测试 RSA 密钥对，返回私钥对象 + PKCS8/PKIX PEM 串。
func genKey(t *testing.T) (*rsa.PrivateKey, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	priv := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pub := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	return key, priv, pub
}

func TestNewOAuthClientNotConfigured(t *testing.T) {
	if c, err := NewOAuthClient(Config{}); err != ErrNotConfigured || c != nil {
		t.Fatalf("空配置应 ErrNotConfigured: %v %v", c, err)
	}
	if c, err := NewOAuthClient(Config{AppID: "x"}); err != ErrNotConfigured || c != nil {
		t.Fatalf("缺私钥应 ErrNotConfigured: %v %v", c, err)
	}
	if _, err := NewOAuthClient(Config{AppID: "x", AppPrivateKey: "not-pem"}); err == nil {
		t.Fatal("非法 PEM 应报错")
	}
}

// canonicalContent 重建支付宝待签内容（key 升序 k=v 用 & 连接）。
func canonicalContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

func TestSignCanonicalAndVerifiable(t *testing.T) {
	key, priv, pub := genKey(t)
	client, err := NewOAuthClient(Config{AppID: "app-1", AppPrivateKey: priv, AlipayPublicKey: pub})
	if err != nil {
		t.Fatal(err)
	}
	params := map[string]string{
		"grant_type": "authorization_code", "code": "auth-code-1", "charset": "utf-8",
		"app_id": "app-1", "sign_type": "RSA2", "method": "alipay.system.oauth.token",
		"format": "JSON", "version": "1.0", "timestamp": "2026-10-25 10:00:00",
	}
	sig := client.sign(params)
	// 确定性：PKCS1v15 同内容同私钥签名恒定
	if sig2 := client.sign(params); sig != sig2 {
		t.Fatal("sign 应确定")
	}
	// 用公钥对重建的待签内容验签（验证 k=v& 拼接规则正确）
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(canonicalContent(params)))
	if err = rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], raw); err != nil {
		t.Fatalf("签名验签失败（拼接规则错误？）: %v", err)
	}
	// 内容篡改 → 验签失败
	bad := sha256.Sum256([]byte(canonicalContent(params) + "x"))
	if err = rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, bad[:], raw); err == nil {
		t.Fatal("篡改内容应验签失败")
	}
}

func TestExtractResponseNode(t *testing.T) {
	body := []byte(`{"alipay_system_oauth_token_response":{"user_id":"u1","access_token":"at"},"sign":"sig"}`)
	node, err := extractResponseNode(body)
	if err != nil || string(node) != `{"user_id":"u1","access_token":"at"}` {
		t.Fatalf("成功节点提取错误: %q %v", node, err)
	}
	errBody := []byte(`{"error_response":{"code":"40002","msg":"x"},"sign":"s"}`)
	node, err = extractResponseNode(errBody)
	if err != nil || string(node) != `{"code":"40002","msg":"x"}` {
		t.Fatalf("错误节点提取错误: %q %v", node, err)
	}
	if _, err = extractResponseNode([]byte(`{"foo":1}`)); err == nil {
		t.Fatal("无内容节点应报错")
	}
}

func TestVerifySkipsWhenNoPublicKey(t *testing.T) {
	_, priv, _ := genKey(t)
	client, err := NewOAuthClient(Config{AppID: "app-1", AppPrivateKey: priv})
	if err != nil {
		t.Fatal(err)
	}
	// 无公钥 → 开发模式，跳过验签
	if err = client.verify([]byte(`{"x":1}`), ""); err != nil {
		t.Fatalf("无公钥应跳过验签: %v", err)
	}
}

func TestExchangeTokenArgValidation(t *testing.T) {
	var nilClient *OAuthClient
	if _, err := nilClient.ExchangeToken(t.Context(), &thirdparty.GetAliPayOauthTokenReq{}); err != ErrNotConfigured {
		t.Fatal("nil 客户端应 ErrNotConfigured")
	}
	_, priv, _ := genKey(t)
	client, err := NewOAuthClient(Config{AppID: "app-1", AppPrivateKey: priv})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.ExchangeToken(t.Context(), &thirdparty.GetAliPayOauthTokenReq{
		GrantType: thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_AUTHORIZATION_CODE,
		AuthCode:  "  ",
	}); err == nil || !strings.Contains(err.Error(), "auth_code") {
		t.Fatalf("空 auth_code 应报错: %v", err)
	}
	if _, err = client.ExchangeToken(t.Context(), &thirdparty.GetAliPayOauthTokenReq{
		GrantType:    thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_REFRESH_TOKEN,
		RefreshToken: "",
	}); err == nil || !strings.Contains(err.Error(), "refresh_token") {
		t.Fatalf("空 refresh_token 应报错: %v", err)
	}
	if _, err = client.ExchangeToken(t.Context(), &thirdparty.GetAliPayOauthTokenReq{
		GrantType: thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_UNSPECIFIED,
	}); err == nil || !strings.Contains(err.Error(), "grant_type") {
		t.Fatalf("非法 grant_type 应报错: %v", err)
	}
}

// alipaySigner 用支付宝私钥对内容节点原始字节签名（mock 服务端）。
func alipaySigner(key *rsa.PrivateKey, node []byte) string {
	digest := sha256.Sum256(node)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func TestExchangeTokenRoundtrip(t *testing.T) {
	alipayKey, _, alipayPub := genKey(t) // 支付宝侧密钥对
	_, appPriv, _ := genKey(t)           // 应用侧密钥对

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "auth-code-1" {
			t.Errorf("请求参数错误: %v", r.Form)
		}
		if r.Form.Get("sign") == "" || r.Form.Get("app_id") != "app-1" || r.Form.Get("sign_type") != "RSA2" {
			t.Errorf("签名/公共参数缺失: %v", r.Form)
		}
		node := `{"user_id":"u-1","access_token":"at-1","refresh_token":"rt-1","open_id":"oid-1"}`
		sign := alipaySigner(alipayKey, []byte(node))
		resp := `{"alipay_system_oauth_token_response":` + node + `,"sign":"` + sign + `"}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer mock.Close()

	client, err := NewOAuthClient(Config{
		AppID: "app-1", AppPrivateKey: appPriv, AlipayPublicKey: alipayPub, APIBase: mock.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.ExchangeToken(t.Context(), &thirdparty.GetAliPayOauthTokenReq{
		GrantType: thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_AUTHORIZATION_CODE,
		AuthCode:  " auth-code-1 ",
	})
	if err != nil {
		t.Fatalf("换取令牌失败: %v", err)
	}
	if resp.GetUserId() != "u-1" || resp.GetAccessToken() != "at-1" || resp.GetRefreshToken() != "rt-1" ||
		resp.GetOpenId() != "oid-1" || resp.GetUnionId() != "oid-1" {
		t.Fatalf("响应字段错误（union_id 应回填 open_id）: %+v", resp)
	}
}

func TestExchangeTokenErrorAndTamper(t *testing.T) {
	alipayKey, _, alipayPub := genKey(t)
	_, appPriv, _ := genKey(t)

	// 错误响应 → 带 code/sub_code 的错误
	mockErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		node := `{"code":"40002","msg":"invalid auth code","sub_code":"isv.code-invalid","sub_msg":"auth code 无效"}`
		sign := alipaySigner(alipayKey, []byte(node))
		_, _ = w.Write([]byte(`{"error_response":` + node + `,"sign":"` + sign + `"}`))
	}))
	defer mockErr.Close()
	clientErr, err := NewOAuthClient(Config{
		AppID: "app-1", AppPrivateKey: appPriv, AlipayPublicKey: alipayPub, APIBase: mockErr.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = clientErr.ExchangeToken(t.Context(), &thirdparty.GetAliPayOauthTokenReq{
		GrantType: thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_AUTHORIZATION_CODE,
		AuthCode:  "bad",
	})
	if err == nil || !strings.Contains(err.Error(), "alipay oauth error 40002/isv.code-invalid") {
		t.Fatalf("错误响应应映射错误: %v", err)
	}

	// 签名篡改 → 验签失败
	mockBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherKey, _, _ := genKey(t) // 非支付宝私钥签名
		node := `{"user_id":"u-1","access_token":"at-1"}`
		_, _ = w.Write([]byte(`{"alipay_system_oauth_token_response":` + node + `,"sign":"` + alipaySigner(otherKey, []byte(node)) + `"}`))
	}))
	defer mockBad.Close()
	clientBad, err := NewOAuthClient(Config{
		AppID: "app-1", AppPrivateKey: appPriv, AlipayPublicKey: alipayPub, APIBase: mockBad.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = clientBad.ExchangeToken(t.Context(), &thirdparty.GetAliPayOauthTokenReq{
		GrantType: thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_AUTHORIZATION_CODE,
		AuthCode:  "ok",
	})
	if err == nil || !strings.Contains(err.Error(), "verify alipay response sign") {
		t.Fatalf("篡改签名应验签失败: %v", err)
	}

	// 空响应体 → 显式报错
	mockEmpty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"sign":"s"}`))
	}))
	defer mockEmpty.Close()
	clientEmpty, err := NewOAuthClient(Config{AppID: "app-1", AppPrivateKey: appPriv, APIBase: mockEmpty.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = clientEmpty.ExchangeToken(t.Context(), &thirdparty.GetAliPayOauthTokenReq{
		GrantType: thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_AUTHORIZATION_CODE,
		AuthCode:  "ok",
	})
	if err == nil || !strings.Contains(err.Error(), "empty alipay oauth response") {
		t.Fatalf("空响应应报错: %v", err)
	}
}

func TestJSONTagShape(t *testing.T) {
	// tokenResponse JSON 形状与支付宝响应一致（tag 漂移防护）
	data := []byte(`{"alipay_system_oauth_token_response":{"user_id":"u","access_token":"a","refresh_token":"r","open_id":"o"},"sign":"s"}`)
	var rsp tokenResponse
	if err := json.Unmarshal(data, &rsp); err != nil {
		t.Fatal(err)
	}
	if rsp.AlipaySystemOauthTokenResponse == nil || rsp.AlipaySystemOauthTokenResponse.OpenId != "o" ||
		rsp.Sign != "s" || rsp.ErrorResponse != nil {
		t.Fatalf("响应形状解析错误: %+v", rsp)
	}
}
