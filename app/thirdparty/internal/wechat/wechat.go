// Package wechat 微信集成（M15a + M15d 全集）：照抄 server_v2 integration/wechat
// 的 ExchangeCode（Mini=jscode2session / App=oauth2 access_token，APIBase/OAuthBase 可配）
// 与小程序全家（MiniProgramAccessToken 本地 2h 缓存 + Redis token store；手机号
// errcode 40001/42001 清 token 重试一次；二维码 scene 必填/DefaultPage 兜底/width 0→430；
// urllink path 必填回 url_link）。
package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"gacha_srv/app/thirdparty/thirdparty"
)

var (
	ErrNotConfigured  = errors.New("wechat integration is not configured")
	ErrInvalidRequest = errors.New("invalid wechat request")
	ErrWeChatAPI      = errors.New("wechat api error")
)

type MiniProgramConfig struct {
	AppID       string
	AppSecret   string
	APIBase     string
	DefaultPage string
}

type MobileAppConfig struct {
	AppID     string
	AppSecret string
	OAuthBase string
}

type Config struct {
	Mini MiniProgramConfig
	App  MobileAppConfig
}

func (c Config) MiniConfigured() bool {
	return c.Mini.AppID != "" && c.Mini.AppSecret != ""
}

func (c Config) AppConfigured() bool {
	return c.App.AppID != "" && c.App.AppSecret != ""
}

type Gateway struct {
	cfg    Config
	tokens *TokenStore
	http   *http.Client

	// 小程序 access_token 本地缓存（2h，server_v2 同款；Redis 为跨实例共享层）
	mu               sync.Mutex
	cachedMiniToken  string
	cachedMiniExpiry time.Time
}

func NewGateway(cfg Config, tokens *TokenStore) *Gateway {
	return &Gateway{
		cfg:    cfg,
		tokens: tokens,
		http:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *Gateway) ExchangeCode(ctx context.Context, scene thirdparty.WeChatOAuthScene, code string) (openID, unionID, sessionKey string, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", "", "", ErrInvalidRequest
	}
	switch scene {
	case thirdparty.WeChatOAuthScene_WECHAT_OAUTH_SCENE_MINI_PROGRAM:
		if !g.cfg.MiniConfigured() {
			return "", "", "", ErrNotConfigured
		}
		base := normalizeBase(g.cfg.Mini.APIBase, "https://api.weixin.qq.com")
		reply, err := g.httpGet(ctx, base+"/sns/jscode2session", url.Values{
			"appid":      {g.cfg.Mini.AppID},
			"secret":     {g.cfg.Mini.AppSecret},
			"js_code":    {code},
			"grant_type": {"authorization_code"},
		})
		if err != nil {
			return "", "", "", err
		}
		return parseOpenID(reply)
	case thirdparty.WeChatOAuthScene_WECHAT_OAUTH_SCENE_APP:
		if !g.cfg.AppConfigured() {
			return "", "", "", ErrNotConfigured
		}
		base := normalizeBase(g.cfg.App.OAuthBase, "https://api.weixin.qq.com")
		reply, err := g.httpGet(ctx, base+"/sns/oauth2/access_token", url.Values{
			"appid":      {g.cfg.App.AppID},
			"secret":     {g.cfg.App.AppSecret},
			"code":       {code},
			"grant_type": {"authorization_code"},
		})
		if err != nil {
			return "", "", "", err
		}
		openID, unionID, _, err = parseOpenID(reply)
		return openID, unionID, "", err
	default:
		return "", "", "", ErrInvalidRequest
	}
}

// MiniProgramAccessToken 小程序 access_token（server_v2 同款三层）：
// 本地缓存（2h）→ Redis token store → /cgi-bin/token（expires_in 减 60s 余量）。
func (g *Gateway) MiniProgramAccessToken(ctx context.Context) (string, error) {
	if !g.cfg.MiniConfigured() {
		return "", ErrNotConfigured
	}
	if token := g.localMiniToken(); token != "" {
		return token, nil
	}
	if g.tokens != nil {
		if token, err := g.tokens.GetMiniAccessToken(ctx); err != nil {
			return "", err
		} else if token != "" {
			g.setLocalMiniToken(token, 2*time.Hour)
			return token, nil
		}
	}
	base := normalizeBase(g.cfg.Mini.APIBase, "https://api.weixin.qq.com")
	reply, err := g.httpGet(ctx, base+"/cgi-bin/token", url.Values{
		"grant_type": {"client_credential"},
		"appid":      {g.cfg.Mini.AppID},
		"secret":     {g.cfg.Mini.AppSecret},
	})
	if err != nil {
		return "", err
	}
	token, ttl, err := parseAccessToken(reply)
	if err != nil {
		return "", err
	}
	g.setLocalMiniToken(token, ttl)
	if g.tokens != nil {
		_ = g.tokens.SetMiniAccessToken(ctx, token, ttl)
	}
	return token, nil
}

func (g *Gateway) localMiniToken() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cachedMiniToken != "" && time.Now().Before(g.cachedMiniExpiry) {
		return g.cachedMiniToken
	}
	return ""
}

func (g *Gateway) setLocalMiniToken(token string, ttl time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cachedMiniToken = token
	g.cachedMiniExpiry = time.Now().Add(ttl)
}

func (g *Gateway) clearLocalMiniToken() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cachedMiniToken = ""
	g.cachedMiniExpiry = time.Time{}
}

// MiniProgramPhoneNumber 小程序手机号（server_v2 同款）：phone_info.purePhoneNumber
// 优先，空则 phoneNumber；errcode 40001/42001 → 清 token（本地+Redis）重试一次。
func (g *Gateway) MiniProgramPhoneNumber(ctx context.Context, phoneCode string) (phone, country string, err error) {
	phoneCode = strings.TrimSpace(phoneCode)
	if phoneCode == "" {
		return "", "", ErrInvalidRequest
	}
	return g.miniProgramPhoneNumber(ctx, phoneCode, false)
}

func (g *Gateway) miniProgramPhoneNumber(ctx context.Context, phoneCode string, retried bool) (phone, country string, err error) {
	token, err := g.MiniProgramAccessToken(ctx)
	if err != nil {
		return "", "", err
	}
	base := normalizeBase(g.cfg.Mini.APIBase, "https://api.weixin.qq.com")
	reply, err := g.httpPostJSON(ctx, base+"/wxa/business/getuserphonenumber?access_token="+url.QueryEscape(token), map[string]string{"code": phoneCode})
	if err != nil {
		return "", "", err
	}
	if errCode := jsonInt(reply, "errcode"); errCode != 0 {
		if !retried && (errCode == 40001 || errCode == 42001) {
			g.clearLocalMiniToken()
			if g.tokens != nil {
				_ = g.tokens.DeleteMiniAccessToken(ctx)
			}
			return g.miniProgramPhoneNumber(ctx, phoneCode, true)
		}
		return "", "", fmt.Errorf("%w: errcode=%d errmsg=%s", ErrWeChatAPI, errCode, jsonString(reply, "errmsg"))
	}
	info, _ := reply["phone_info"].(map[string]any)
	if info == nil {
		return "", "", ErrWeChatAPI
	}
	phone = jsonString(info, "purePhoneNumber")
	if phone == "" {
		phone = jsonString(info, "phoneNumber")
	}
	country = jsonString(info, "countryCode")
	if phone == "" {
		return "", "", ErrWeChatAPI
	}
	return phone, country, nil
}

// GenerateMiniProgramQrCode 小程序码（server_v2 同款）：scene 必填、page 空用
// DefaultPage（仍空 → 报错）、width 0→430、响应 JSON（`{` 开头且有 errmsg）→ API 错误。
func (g *Gateway) GenerateMiniProgramQrCode(ctx context.Context, scene, page string, width uint32, checkPath bool) ([]byte, error) {
	if !g.cfg.MiniConfigured() {
		return nil, ErrNotConfigured
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return nil, ErrInvalidRequest
	}
	page = strings.TrimSpace(page)
	if page == "" {
		page = strings.TrimSpace(g.cfg.Mini.DefaultPage)
	}
	if page == "" {
		return nil, fmt.Errorf("%w: mini program page is required", ErrInvalidRequest)
	}
	if width == 0 {
		width = 430
	}
	token, err := g.MiniProgramAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	base := normalizeBase(g.cfg.Mini.APIBase, "https://api.weixin.qq.com")
	body := map[string]any{
		"scene":      scene,
		"page":       page,
		"width":      width,
		"check_path": checkPath,
	}
	raw, err := g.httpPostRaw(ctx, base+"/wxa/getwxacodeunlimit?access_token="+url.QueryEscape(token), body)
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 && raw[0] == '{' {
		var reply map[string]any
		if json.Unmarshal(raw, &reply) == nil {
			if msg := jsonString(reply, "errmsg"); msg != "" {
				return nil, fmt.Errorf("%w: %s", ErrWeChatAPI, msg)
			}
		}
		return nil, ErrWeChatAPI
	}
	return raw, nil
}

// GenerateMiniProgramUrlLink 小程序 URL Link（server_v2 同款）：path 必填、回 url_link。
func (g *Gateway) GenerateMiniProgramUrlLink(ctx context.Context, path, query string) (string, error) {
	if !g.cfg.MiniConfigured() {
		return "", ErrNotConfigured
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ErrInvalidRequest
	}
	token, err := g.MiniProgramAccessToken(ctx)
	if err != nil {
		return "", err
	}
	base := normalizeBase(g.cfg.Mini.APIBase, "https://api.weixin.qq.com")
	reply, err := g.httpPostJSON(ctx, base+"/wxa/generate_urllink?access_token="+url.QueryEscape(token), map[string]string{
		"path":  path,
		"query": query,
	})
	if err != nil {
		return "", err
	}
	if errCode := jsonInt(reply, "errcode"); errCode != 0 {
		return "", fmt.Errorf("%w: errcode=%d errmsg=%s", ErrWeChatAPI, errCode, jsonString(reply, "errmsg"))
	}
	link := jsonString(reply, "url_link")
	if link == "" {
		return "", ErrWeChatAPI
	}
	return link, nil
}

func (g *Gateway) httpGet(ctx context.Context, endpoint string, query url.Values) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeWeChatJSON(resp.Body)
}

// httpPostJSON POST JSON 并解析响应对象。
func (g *Gateway) httpPostJSON(ctx context.Context, endpoint string, payload any) (map[string]any, error) {
	raw, err := g.httpPostRaw(ctx, endpoint, payload)
	if err != nil {
		return nil, err
	}
	var reply map[string]any
	if err = json.Unmarshal(raw, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// httpPostRaw POST JSON 返回原始字节（二维码图片用）。
func (g *Gateway) httpPostRaw(ctx context.Context, endpoint string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// parseAccessToken 解析 /cgi-bin/token 应答：expires_in>60 才减 60s 余量，否则 2h（server_v2 同款）。
func parseAccessToken(reply map[string]any) (token string, ttl time.Duration, err error) {
	token = jsonString(reply, "access_token")
	if token == "" {
		if errCode := jsonInt(reply, "errcode"); errCode != 0 {
			return "", 0, fmt.Errorf("%w: errcode=%d errmsg=%s", ErrWeChatAPI, errCode, jsonString(reply, "errmsg"))
		}
		return "", 0, ErrWeChatAPI
	}
	ttl = 2 * time.Hour
	if sec := jsonInt(reply, "expires_in"); sec > 60 {
		ttl = time.Duration(sec-60) * time.Second
	}
	return token, ttl, nil
}

// decodeWeChatJSON 解析微信 JSON 应答；errcode≠0 直接报 ErrWeChatAPI
func decodeWeChatJSON(r io.Reader) (map[string]any, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var reply map[string]any
	if err = json.Unmarshal(raw, &reply); err != nil {
		return nil, err
	}
	if errCode := jsonInt(reply, "errcode"); errCode != 0 {
		return nil, fmt.Errorf("%w: errcode=%d errmsg=%s", ErrWeChatAPI, errCode, jsonString(reply, "errmsg"))
	}
	return reply, nil
}

func parseOpenID(reply map[string]any) (openID, unionID, sessionKey string, err error) {
	openID = jsonString(reply, "openid")
	if openID == "" {
		if errCode := jsonInt(reply, "errcode"); errCode != 0 {
			return "", "", "", fmt.Errorf("%w: errcode=%d errmsg=%s", ErrWeChatAPI, errCode, jsonString(reply, "errmsg"))
		}
		return "", "", "", ErrWeChatAPI
	}
	unionID = jsonString(reply, "unionid")
	if unionID == "" {
		unionID = openID
	}
	sessionKey = jsonString(reply, "session_key")
	return openID, unionID, sessionKey, nil
}

func normalizeBase(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	return strings.TrimRight(raw, "/")
}

func jsonString(v map[string]any, key string) string {
	raw, ok := v[key]
	if !ok || raw == nil {
		return ""
	}
	switch t := raw.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	default:
		return fmt.Sprint(t)
	}
}

func jsonInt(v map[string]any, key string) int64 {
	s := jsonString(v, key)
	if s == "" {
		return 0
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
