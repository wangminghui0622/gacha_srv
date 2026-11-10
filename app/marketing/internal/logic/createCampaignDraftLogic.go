package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/marketing/internal/core"
	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type CreateCampaignDraftLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCampaignDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCampaignDraftLogic {
	return &CreateCampaignDraftLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ── M14d 控制台（campaign+promotion 同属 game 域；消息形状照抄 server_v2，见文末）──
// CreateCampaignDraft 设计 D1：CampaignId==0 新建（head+key 冲突 5015 + v1 DRAFT）；
// CampaignId!=0 追加草稿（锁 head，key/kind 不符 5015，latest 仍 DRAFT → 草稿上加草稿 5015）。
func (l *CreateCampaignDraftLogic) CreateCampaignDraft(in *marketing.CreateCampaignDraftRequest) (*marketing.Campaign, error) {
	if err := core.ValidateCampaignDraft(in); err != nil {
		return nil, errorx.NewCodeErrorf(errorx.CodeCampaignParamInvalid, "%v", err)
	}
	configJSON, err := core.MarshalCampaignConfig(in.Config)
	if err != nil {
		return nil, errorx.NewCodeErrorf(errorx.CodeCampaignParamInvalid, "config 序列化失败: %v", err)
	}
	startAt, endAt := time.UnixMilli(in.StartAtUnixMillis), time.UnixMilli(in.EndAtUnixMillis)

	if in.CampaignId == 0 {
		// 新建：key 已存在 → 冲突
		if _, err := repo.GetCampaignHeadByKey(l.svcCtx.DB, in.CampaignKey); err == nil {
			return nil, errorx.NewCodeError(errorx.CodeCampaignConflict, "campaignKey 已存在")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		head := &repo.CampaignHead{
			CampaignKey: in.CampaignKey,
			Name:        in.Name,
			Kind:        int32(in.Kind),
			ConfigJSON:  "", // 草稿头无物化配置（发布时物化；status=0 玩家不可见）
			StartAt:     startAt,
			EndAt:       endAt,
			Status:      0,
		}
		v := &repo.CampaignVersionRow{
			CampaignID: 0, // 由 head.ID 回填
			Version:    1,
			Kind:       int32(in.Kind),
			Name:       in.Name,
			State:      repo.CampaignStateDraft,
			ConfigJSON: configJSON,
			StartAt:    startAt,
			EndAt:      endAt,
		}
		err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
			if err := repo.InsertCampaignHead(tx, head); err != nil {
				return err
			}
			v.CampaignID = head.ID
			return repo.InsertCampaignVersion(tx, v)
		})
		if err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return nil, errorx.NewCodeError(errorx.CodeCampaignConflict, "campaignKey 已存在")
			}
			return nil, err
		}
		// 玩家侧列表读缓存模型 campaigns，新建草稿 status=0 不可见；无需删缓存。
		return campaignProtoOf(head, v, in.Config, 0), nil
	}

	// 追加草稿
	var head *repo.CampaignHead
	var v *repo.CampaignVersionRow
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		h, err := repo.LockCampaignHead(tx, int64(in.CampaignId))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动不存在")
			}
			return err
		}
		if in.CampaignKey != "" && in.CampaignKey != h.CampaignKey {
			return errorx.NewCodeError(errorx.CodeCampaignConflict, "campaignKey 不符")
		}
		if in.Kind != marketing.CampaignKind(h.Kind) {
			return errorx.NewCodeError(errorx.CodeCampaignConflict, "kind 不符")
		}
		latest, err := repo.LockLatestCampaignVersion(tx, h.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errorx.NewCodeError(errorx.CodeCampaignNotFound, "活动无版本记录")
			}
			return err
		}
		if latest.State == repo.CampaignStateDraft {
			return errorx.NewCodeError(errorx.CodeCampaignConflict, "已有未发布草稿，不能继续追加")
		}
		head = h
		v = &repo.CampaignVersionRow{
			CampaignID: h.ID,
			Version:    latest.Version + 1,
			Kind:       h.Kind,
			Name:       in.Name,
			State:      repo.CampaignStateDraft,
			ConfigJSON: configJSON,
			StartAt:    startAt,
			EndAt:      endAt,
		}
		return repo.InsertCampaignVersion(tx, v)
	})
	if err != nil {
		return nil, err
	}
	return campaignProtoOf(head, v, in.Config, head.PublishedVersion), nil
}
