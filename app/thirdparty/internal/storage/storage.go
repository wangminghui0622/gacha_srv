// Package storage 对象存储上传接口：OSS / COS 二选一。
package storage

import "context"

// Uploader 统一的图片/文件上传能力。
type Uploader interface {
	Enabled() bool
	ObjectKey(filename, dir string) string
	Upload(ctx context.Context, objectKey string, data []byte, contentType string) (string, error)
}
