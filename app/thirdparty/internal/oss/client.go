// Package oss 封装阿里云 OSS 上传（与 house/admin-back 的 OSSService 行为一致）：
// PutObject + public-read + 一年缓存，返回公开访问 URL。
package oss

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

var ErrNotConfigured = errors.New("oss: not configured")

type Config struct {
	Enabled         bool
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	PublicBaseURL   string
	Dir             string
}

type Client struct {
	cfg    Config
	bucket *oss.Bucket
}

// New 创建 OSS 客户端；未启用/配置不完整时返回可用但 Disabled 的客户端。
func New(cfg Config) (*Client, error) {
	enabled := cfg.Enabled &&
		strings.TrimSpace(cfg.Endpoint) != "" &&
		strings.TrimSpace(cfg.Bucket) != "" &&
		strings.TrimSpace(cfg.AccessKeyID) != "" &&
		strings.TrimSpace(cfg.AccessKeySecret) != ""
	if !enabled {
		return &Client{cfg: cfg}, nil
	}
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret,
		oss.Timeout(30, 60),
	)
	if err != nil {
		return nil, fmt.Errorf("oss: create client: %w", err)
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("oss: get bucket: %w", err)
	}
	return &Client{cfg: cfg, bucket: bucket}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.bucket != nil
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

// Upload 上传字节到 OSS，返回对象 key 与可访问 URL。
func (c *Client) Upload(ctx context.Context, objectKey string, data []byte, contentType string) (string, error) {
	if !c.Enabled() {
		return "", ErrNotConfigured
	}
	if len(data) == 0 {
		return "", errors.New("oss: empty file")
	}
	objectKey = strings.TrimPrefix(strings.TrimSpace(objectKey), "/")
	if objectKey == "" {
		return "", errors.New("oss: empty object key")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	err := c.bucket.PutObject(objectKey, bytes.NewReader(data),
		oss.ContentType(contentType),
		oss.ObjectACL(oss.ACLPublicRead),
		oss.CacheControl("max-age=31536000"),
	)
	if err != nil {
		return "", fmt.Errorf("oss: put object: %w", err)
	}

	publicBase := strings.TrimRight(strings.TrimSpace(c.cfg.PublicBaseURL), "/")
	if publicBase != "" {
		return publicBase + "/" + escapePath(objectKey), nil
	}
	endpoint := strings.TrimSpace(c.cfg.Endpoint)
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	return "https://" + c.cfg.Bucket + "." + strings.TrimRight(endpoint, "/") + "/" + escapePath(objectKey), nil
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

func escapePath(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
