package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/core"
	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetCampaignLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCampaignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCampaignLogic {
	return &GetCampaignLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCampaign 设计 D1：published_version>0 读该版本行；=0 读 latest（未发布草稿读最新版）。
func (l *GetCampaignLogic) GetCampaign(in *marketing.GetCampaignRequest) (*marketing.Campaign, error) {
	if in.TenantId == 0 || in.CampaignId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "tenantId/campaignId 必须大于 0")
	}
	// 读路径无事务：head 快照读（head 行是控制台低频对象，无缓存模型参与）
	head := &repo.CampaignHead{ID: int64(in.CampaignId)}
	if err := l.svcCtx.DB.First(head).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动不存在")
		}
		return nil, err
	}
	var v *repo.CampaignVersionRow
	var err error
	if head.PublishedVersion > 0 {
		v, err = repo.GetCampaignVersion(l.svcCtx.DB, head.ID, head.PublishedVersion)
	} else {
		v, err = repo.GetLatestCampaignVersion(l.svcCtx.DB, head.ID)
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动无版本记录")
		}
		return nil, err
	}
	config, err := core.UnmarshalCampaignConfig(v.ConfigJSON)
	if err != nil {
		return nil, errorx.NewCodeErrorf(errorx.CodeCampaignParamInvalid, "存储配置解析失败: %v", err)
	}
	return campaignProtoOf(head, v, config, head.PublishedVersion), nil
}
