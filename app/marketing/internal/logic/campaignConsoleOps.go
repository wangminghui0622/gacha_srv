package logic

import (
	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/marketing"
)

// campaignProtoOf head+version+config → console Campaign 响应（3 个活动 RPC 共用）。
// tenantID 单租户固定 1（设计 D7，wire 兼容保留字段）。
func campaignProtoOf(head *repo.CampaignHead, v *repo.CampaignVersionRow,
	cfg *marketing.CampaignConfig, publishedVersion int64) *marketing.Campaign {
	return &marketing.Campaign{
		TenantId:          1,
		CampaignId:        uint64(head.ID),
		CampaignKey:       head.CampaignKey,
		Kind:              marketing.CampaignKind(v.Kind),
		Name:              v.Name,
		State:             marketing.CampaignState(v.State),
		Version:           uint32(v.Version),
		StartAtUnixMillis: v.StartAt.UnixMilli(),
		EndAtUnixMillis:   v.EndAt.UnixMilli(),
		Config:            cfg,
		PublishedVersion:  uint32(publishedVersion),
	}
}
