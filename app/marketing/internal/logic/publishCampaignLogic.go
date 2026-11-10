package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/core"
	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	gamemodel "gacha_srv/pkg/model/gacha_marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type PublishCampaignLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishCampaignLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishCampaignLogic {
	return &PublishCampaignLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishCampaign 设计 D1/D2：锁 head；latest 非 DRAFT → closed 冲突（无重放幂等，
// 已发布再发 → 5015）；物化 player 形状到 campaigns 行（published_version CAS）+
// 兑换码 upsert / 转盘槽全删全插，单事务。
func (l *PublishCampaignLogic) PublishCampaign(in *marketing.PublishCampaignRequest) (*marketing.Campaign, error) {
	if in.TenantId == 0 || in.CampaignId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeCampaignParamInvalid, "tenantId/campaignId 必须大于 0")
	}
	var (
		head   *repo.CampaignHead
		v      *repo.CampaignVersionRow
		config *marketing.CampaignConfig
	)
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		head, err = repo.LockCampaignHead(tx, int64(in.CampaignId))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动不存在")
			}
			return err
		}
		v, err = repo.LockLatestCampaignVersion(tx, head.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动无版本记录")
			}
			return err
		}
		if v.State != repo.CampaignStateDraft {
			return errorx.NewCodeError(errorx.CodeCampaignConflict, "最新版本已发布，不能重复发布")
		}
		config, err = core.UnmarshalCampaignConfig(v.ConfigJSON)
		if err != nil {
			return errorx.NewCodeErrorf(errorx.CodeCampaignParamInvalid, "存储配置解析失败: %v", err)
		}
		mat, err := core.MaterializeCampaignConfig(marketing.CampaignKind(v.Kind), config)
		if err != nil {
			return errorx.NewCodeErrorf(errorx.CodeCampaignParamInvalid, "%v", err)
		}

		// 物化 head（player 形状 + published_version CAS）
		ok, err := repo.PublishCampaignHeadCAS(tx, head.ID, head.PublishedVersion, v.Version,
			v.Name, mat.ConfigJSON, v.StartAt, v.EndAt)
		if err != nil {
			return err
		}
		if !ok {
			return errorx.NewCodeError(errorx.CodeCampaignConflict, "published_version 不符，活动状态已变更")
		}
		if err := repo.MarkCampaignVersionPublished(tx, head.ID, v.Version); err != nil {
			return err
		}

		// 兑换码/转盘槽物化（player 引擎读这两张表）
		switch marketing.CampaignKind(v.Kind) {
		case marketing.CampaignKind_CAMPAIGN_KIND_REDEMPTION_CODE:
			keep := make([]string, 0, len(mat.Codes))
			for _, c := range mat.Codes {
				keep = append(keep, c.Code)
				if err := repo.UpsertRedemptionCode(tx, &repo.RedemptionCodeRow{
					CampaignID:     head.ID,
					Code:           c.Code,
					MaxRedemptions: c.MaxRedemptions,
					PerUserLimit:   c.PerUserLimit,
					PrizeConfig:    c.PrizeConfig,
				}); err != nil {
					return err
				}
			}
			if err := repo.DisableRedemptionCodesExcept(tx, head.ID, keep); err != nil {
				return err
			}
		case marketing.CampaignKind_CAMPAIGN_KIND_PRIZE_WHEEL:
			if err := repo.DeleteWheelSlots(tx, head.ID); err != nil {
				return err
			}
			for _, s := range mat.Slots {
				if err := repo.InsertWheelSlot(tx, &repo.WheelSlotRow{
					CampaignID:       head.ID,
					SlotIndex:        s.SlotIndex,
					Label:            s.Label,
					Weight:           s.Weight,
					PrizeKind:        s.PrizeKind,
					CashFen:          s.CashFen,
					PointsAmount:     s.PointsAmount,
					CouponTemplateNo: s.CouponTemplateNo,
					PrizeConfig:      s.PrizeConfig,
					DailyStock:       s.DailyStock,
					Remaining:        s.Remaining,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// raw UPDATE 绕过缓存模型：commit 后手动失效（best-effort，兜底靠 30s TTL）
	if e := l.svcCtx.CampaignModel.DeleteCache(l.ctx, &gamemodel.Campaigns{
		Id:          head.ID,
		CampaignKey: head.CampaignKey,
	}); e != nil {
		l.Errorf("PublishCampaign DeleteCache failed, campaignId: %d, err: %v", head.ID, e)
	}
	v.State = repo.CampaignStatePublished
	return campaignProtoOf(head, v, config, v.Version), nil
}
