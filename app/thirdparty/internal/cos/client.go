// Package cos 封装腾讯云 COS 上传：PutObject + public-read + 一年缓存，返回公开访问 URL。
package cos

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	tencentcos "github.com/tencentyun/cos-go-sdk-v5"
)

var ErrNotConfigured = errors.New("cos: not configured")

type Config struct {
	Enabled       bool
	Endpoint      string // https://<bucket>.cos.<region>.myqcloud.com
	SecretID      string
	SecretKey     string
	PublicBaseURL string // 可选 CDN/自定义域名
	Dir           string // 默认目录前缀
}

type Client struct {
	cfg    Config
	client *tencentcos.Client
}

// New 创建 COS 客户端；未启用/配置不完整时返回可用但 Disabled 的客户端。
func New(cfg Config) (*Client, error) {
	enabled := cfg.Enabled &&
		strings.TrimSpace(cfg.Endpoint) != "" &&
		strings.TrimSpace(cfg.SecretID) != "" &&
		strings.TrimSpace(cfg.SecretKey) != ""
	if !enabled {
		return &Client{cfg: cfg}, nil
	}
	endpoint, err := url.Parse(strings.TrimSpace(cfg.Endpoint))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("cos: invalid endpoint %q", cfg.Endpoint)
	}
	baseURL := &tencentcos.BaseURL{BucketURL: endpoint}
	client := tencentcos.NewClient(baseURL, &http.Client{
		Transport: &tencentcos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})
	return &Client{cfg: cfg, client: client}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.client != nil
}

// ObjectKey 生成带日期和随机串的对象 key，避免文件名冲突。
func (c *Client) ObjectKey(filename, dir string) string {
	dir = sanitizeDir(dir)
	if dir == "" {
		dir = sanitizeDir(c.cfg.Dir)
	}
	if dir == "" {
		dir = "gacha"
	}
	ext := strings.ToLower(path.Ext(strings.TrimSpace(filename)))
	if len(ext) > 16 {
		ext = ""
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		random = [8]byte{byte(time.Now().UnixNano())}
	}
	return dir + "/" + time.Now().UTC().Format("2006/01/02") + "/" + hex.EncodeToString(random[:]) + ext
}

// Upload 上传字节到 COS，返回可访问 URL。
func (c *Client) Upload(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {
	if !c.Enabled() {
		return "", ErrNotConfigured
	}
	if len(data) == 0 {
		return "", errors.New("cos: empty file")
	}
	objectKey = strings.TrimPrefix(strings.TrimSpace(objectKey), "/")
	if objectKey == "" {
		return "", errors.New("cos: empty object key")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	opt := &tencentcos.ObjectPutOptions{
		ACLHeaderOptions: &tencentcos.ACLHeaderOptions{
			XCosACL: "public-read",
		},
		ObjectPutHeaderOptions: &tencentcos.ObjectPutHeaderOptions{
			ContentType:  contentType,
			CacheControl: "max-age=31536000",
		},
	}
	// Docker 内置 DNS 偶发 "server misbehaving"，对网络类错误做带间隔重试。
	var putErr error
	for attempt := 0; attempt < 3; attempt++ {
		if _, err := c.client.Object.Put(ctx, objectKey, bytes.NewReader(data), opt); err == nil {
			putErr = nil
			break
		} else {
			putErr = err
			if !retryableUploadError(err) || attempt == 2 {
				break
			}
			delay := time.Duration(200*(1<<attempt)) * time.Millisecond
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("cos: put object: %w", ctx.Err())
			case <-time.After(delay):
			}
		}
	}
	if putErr != nil {
		return "", fmt.Errorf("cos: put object: %w", putErr)
	}

	publicBase := strings.TrimRight(strings.TrimSpace(c.cfg.PublicBaseURL), "/")
	if publicBase != "" {
		return publicBase + "/" + escapePath(objectKey), nil
	}
	return strings.TrimRight(strings.TrimSpace(c.cfg.Endpoint), "/") + "/" + escapePath(objectKey), nil
}

func sanitizeDir(dir string) string {
	dir = strings.Trim(strings.TrimSpace(dir), "/")
	if dir == "" || strings.Contains(dir, "..") {
		return ""
	}
	parts := strings.Split(dir, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, "/")
}

func retryableUploadError(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

func escapePath(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
