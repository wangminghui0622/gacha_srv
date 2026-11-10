// Package tencentad 腾讯广告点击记录（M15d）。
// 语义照抄 server_v2 integration internal/tencentad/ad_click.go 的 RecordClick：
//   - wechat_open_id 必填（缺失报错 → errorx 6010）；
//   - 同 openid 已存在则跳过（uk wechat_open_id 仲裁，重复上报零副作用）；
//   - click_time 字符串解析（parseClickTime）。
//
// 偏离（M15d D4）：ReportAction 死代码不克隆（依赖腾讯营销 SDK 与外部上报，
// 老项目无任何调用方）；ID 用 AUTO_INCREMENT（D5），AdClick 不含 ID 字段。
package tencentad

import (
	"context"
	"errors"
	"strconv"
)

// AdClick 落库行（不含 ID，AUTO_INCREMENT 由 store 回填）。
type AdClick struct {
	TenantID     uint64
	AccountID    string
	ClickID      string
	ClickTime    int64
	WechatOpenID string
	DeviceOsType string
}

// AdClickStore 存储抽象（repo.TxAdClickStore 实现）。
type AdClickStore interface {
	CountByOpenID(ctx context.Context, openID string) (int64, error)
	Insert(ctx context.Context, row *AdClick) error
}

// AdClickMessage RPC 入参（click_time 是 proto 字符串，落库前解析）。
type AdClickMessage struct {
	TenantID     uint64
	AccountID    string
	ClickID      string
	ClickTime    string
	WechatOpenID string
	DeviceOsType string
}

// Service 广告点击服务（server_v2 tencentad.Service 去掉 ID 生成与 ReportAction）。
type Service struct {
	store AdClickStore
}

// NewService 构建服务。
func NewService(store AdClickStore) *Service {
	return &Service{store: store}
}

// RecordClick 记录点击（server_v2 同款：openid 必填 → 去重 → 落库）。
func (s *Service) RecordClick(ctx context.Context, msg AdClickMessage) error {
	if msg.WechatOpenID == "" {
		return errors.New("wechat_open_id is required")
	}
	count, err := s.store.CountByOpenID(ctx, msg.WechatOpenID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.store.Insert(ctx, &AdClick{
		TenantID:     msg.TenantID,
		AccountID:    msg.AccountID,
		ClickID:      msg.ClickID,
		ClickTime:    parseClickTime(msg.ClickTime),
		WechatOpenID: msg.WechatOpenID,
		DeviceOsType: msg.DeviceOsType,
	})
}

// parseClickTime server_v2 同款：十进制秒级时间戳，解析失败得 0。
func parseClickTime(raw string) int64 {
	n, _ := strconv.ParseInt(raw, 10, 64)
	return n
}
