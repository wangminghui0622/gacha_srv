// Package alipay 支付宝 OAuth（M15d，D3 裁决）：不引 gopay，crypto/rsa 手写 RSA2。
// 语义照抄 server_v2 integration internal/alipay/oauth.go（gopay SystemOauthToken）：
//   - authorization_code：auth_code trim 后必填；refresh_token：refresh_token 必填；
//   - 响应 union_id 回填 open_id；
//   - 未配置（AppID/AppPrivateKey 空）→ ErrNotConfigured（errorx 6007）。
//
// RSA2 实现（支付宝开放平台规范）：参数按 key 升序拼 k=v&… 作为待签内容，
// SHA256withRSA 签名（PKCS1v15），base64 后 form 编码 POST 到网关。
// 验签：AlipayPublicKey 为空 ⇒ 跳过（开发模式，mock 无公钥）；非空则对响应内容节点
// 原始 JSON 字节验签，失败报错。
package alipay

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"gacha_srv/app/thirdparty/thirdparty"
)

// ErrNotConfigured 支付宝 OAuth 未配置（appid/私钥缺失，errorx 6007）。
var ErrNotConfigured = errors.New("alipay oauth is not configured")

// 网关地址（gopay 同款：生产/沙箱；APIBase 显式配置优先，场景测试指向本地 alipaymock）。
const (
	gatewayProduction = "https://openapi.alipay.com/gateway.do"
	gatewaySandbox    = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
)

// alipayLoc 支付宝时间戳时区（东八区固定偏移，不随服务器本地时区漂移）。
var alipayLoc = time.FixedZone("CST", 8*3600)

// Config 支付宝 OAuth 配置（config.Alipay 同款字段）。
type Config struct {
	AppID           string
	AppPrivateKey   string
	AlipayPublicKey string
	Production      bool
	// APIBase 网关基础地址（空=按 Production 选官方/沙箱；场景测试指向本地 mock）
	APIBase string
}

// OAuthClient 支付宝 OAuth 客户端（懒加载：nil 表示未配置，logic 层映射 6007）。
type OAuthClient struct {
	cfg      Config
	endpoint string
	signer   *rsa.PrivateKey
	verifier *rsa.PublicKey
	http     *http.Client
}

// NewOAuthClient 构建客户端；未配置返回 (nil, ErrNotConfigured)（server_v2 provideAlipayOAuth
// 的 nil 检查 + NewOAuthClient 的 ErrNotConfigured 合并表达）。
func NewOAuthClient(cfg Config) (*OAuthClient, error) {
	if cfg.AppID == "" || cfg.AppPrivateKey == "" {
		return nil, ErrNotConfigured
	}
	signer, err := parseRSAPrivateKey(cfg.AppPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse alipay app private key: %w", err)
	}
	var verifier *rsa.PublicKey
	if strings.TrimSpace(cfg.AlipayPublicKey) != "" {
		verifier, err = parseRSAPublicKey(cfg.AlipayPublicKey)
		if err != nil {
			return nil, fmt.Errorf("parse alipay public key: %w", err)
		}
	}
	endpoint := cfg.APIBase
	if endpoint == "" {
		if cfg.Production {
			endpoint = gatewayProduction
		} else {
			endpoint = gatewaySandbox
		}
	}
	return &OAuthClient{
		cfg:      cfg,
		endpoint: endpoint,
		signer:   signer,
		verifier: verifier,
		http:     &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// tokenResponse alipay.system.oauth.token 响应（成功/错误共用 sign 验签结构）。
type tokenResponse struct {
	AlipaySystemOauthTokenResponse *tokenBody `json:"alipay_system_oauth_token_response"`
	ErrorResponse                  *errorBody `json:"error_response"`
	Sign                           string     `json:"sign"`
}

type tokenBody struct {
	UserId       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	OpenId       string `json:"open_id"`
}

type errorBody struct {
	Code    string `json:"code"`
	Msg     string `json:"msg"`
	SubCode string `json:"sub_code"`
	SubMsg  string `json:"sub_msg"`
}

// ExchangeToken 换取用户令牌（server_v2 OAuthClient.ExchangeToken 同款语义）。
func (c *OAuthClient) ExchangeToken(ctx context.Context, req *thirdparty.GetAliPayOauthTokenReq) (*thirdparty.GetAliPayOauthTokenResp, error) {
	if c == nil {
		return nil, ErrNotConfigured
	}
	params := map[string]string{
		"app_id":    c.cfg.AppID,
		"method":    "alipay.system.oauth.token",
		"format":    "JSON",
		"charset":   "utf-8",
		"sign_type": "RSA2",
		"timestamp": time.Now().In(alipayLoc).Format("2006-01-02 15:04:05"),
		"version":   "1.0",
	}
	switch req.GetGrantType() {
	case thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_AUTHORIZATION_CODE:
		code := strings.TrimSpace(req.GetAuthCode())
		if code == "" {
			return nil, errors.New("auth_code is required")
		}
		params["grant_type"] = "authorization_code"
		params["code"] = code
	case thirdparty.AliPayOAuthGrantType_ALIPAY_OAUTH_GRANT_TYPE_REFRESH_TOKEN:
		token := strings.TrimSpace(req.GetRefreshToken())
		if token == "" {
			return nil, errors.New("refresh_token is required")
		}
		params["grant_type"] = "refresh_token"
		params["refresh_token"] = token
	default:
		return nil, errors.New("unsupported grant_type")
	}
	params["sign"] = c.sign(params)

	body, err := c.postForm(ctx, params)
	if err != nil {
		return nil, err
	}
	var rsp tokenResponse
	if err = json.Unmarshal(body, &rsp); err != nil {
		return nil, fmt.Errorf("decode alipay oauth response: %w", err)
	}
	if rsp.ErrorResponse != nil {
		return nil, fmt.Errorf("alipay oauth error %s/%s: %s", rsp.ErrorResponse.Code, rsp.ErrorResponse.SubCode, rsp.ErrorResponse.SubMsg)
	}
	if rsp.AlipaySystemOauthTokenResponse == nil {
		return nil, errors.New("empty alipay oauth response")
	}
	if err = c.verify(body, rsp.Sign); err != nil {
		return nil, err
	}
	body2 := rsp.AlipaySystemOauthTokenResponse
	openID := body2.OpenId
	return &thirdparty.GetAliPayOauthTokenResp{
		UserId:       body2.UserId,
		AccessToken:  body2.AccessToken,
		RefreshToken: body2.RefreshToken,
		OpenId:       openID,
		UnionId:      openID,
	}, nil
}

// sign 支付宝签名：key 升序 k=v 用 & 连接，SHA256withRSA（PKCS1v15），base64。
func (c *OAuthClient) sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	content := strings.Join(parts, "&")
	digest := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.signer, crypto.SHA256, digest[:])
	if err != nil {
		// 配置私钥在 NewOAuthClient 已解析成功，此处失败属环境异常，panic 暴露
		panic("alipay sign: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// verify 验签：公钥未配置跳过（开发模式）；否则对响应内容节点原始字节验签。
func (c *OAuthClient) verify(body []byte, sign string) error {
	if c.verifier == nil {
		return nil
	}
	if sign == "" {
		return errors.New("alipay response sign is missing")
	}
	raw, err := extractResponseNode(body)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return fmt.Errorf("decode alipay response sign: %w", err)
	}
	digest := sha256.Sum256(raw)
	if err = rsa.VerifyPKCS1v15(c.verifier, crypto.SHA256, digest[:], sig); err != nil {
		return fmt.Errorf("verify alipay response sign: %w", err)
	}
	return nil
}

// extractResponseNode 从响应原始字节中抠出内容节点（alipay_system_oauth_token_response
// 或 error_response）的原始 JSON 字节——支付宝验签对象就是该节点的原文。
func extractResponseNode(body []byte) ([]byte, error) {
	for _, key := range []string{`"alipay_system_oauth_token_response"`, `"error_response"`} {
		idx := findJSONValue(body, key)
		if idx < 0 {
			continue
		}
		depth := 0
		for i := idx; i < len(body); i++ {
			switch body[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return body[idx : i+1], nil
				}
			}
		}
	}
	return nil, errors.New("alipay response content node is missing")
}

// findJSONValue 定位 key 后第一个 '{' 的位置（key 需带引号）。
func findJSONValue(body []byte, key string) int {
	idx := strings.Index(string(body), key)
	if idx < 0 {
		return -1
	}
	rest := body[idx+len(key):]
	colon := strings.IndexByte(string(rest), ':')
	if colon < 0 {
		return -1
	}
	brace := strings.IndexByte(string(rest[colon+1:]), '{')
	if brace < 0 {
		return -1
	}
	return idx + len(key) + colon + 1 + brace
}

// postForm 网关 POST（application/x-www-form-urlencoded），读满响应体。
func (c *OAuthClient) postForm(ctx context.Context, params map[string]string) ([]byte, error) {
	form := make(url.Values, len(params))
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alipay gateway: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read alipay response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alipay gateway status %d: %s", resp.StatusCode, truncate(body, 256))
	}
	return body, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}

// parseRSAPrivateKey 支持 PKCS8 / PKCS1（支付宝应用私钥两种常见 PEM 形态）。
func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("private key PEM is missing")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("private key is not RSA")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("unsupported private key format (want PKCS8/PKCS1 RSA)")
}

// parseRSAPublicKey 支持 PKIX / PKCS1（支付宝公钥两种常见 PEM 形态）。
func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("public key PEM is missing")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("public key is not RSA")
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("unsupported public key format (want PKIX/PKCS1 RSA)")
}
