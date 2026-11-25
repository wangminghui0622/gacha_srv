package logic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

var activityDailyTimePattern = regexp.MustCompile(`^\d{2}:\d{2}(:\d{2})?$`)

type UpsertActivityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertActivityLogic {
	return &UpsertActivityLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// UpsertActivity 创建/编辑活动（id=0 创建）。
func (l *UpsertActivityLogic) UpsertActivity(in *marketing.UpsertActivityRequest) (*marketing.Activity, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "tenantId 须为 1（单租户）")
	}
	a := in.Activity
	if a == nil {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "activity 不能为空")
	}
	a.Name = strings.TrimSpace(a.Name)
	a.Description = strings.TrimSpace(a.Description)
	a.Protocol = strings.TrimSpace(a.Protocol)
	a.DailyStart = strings.TrimSpace(a.DailyStart)
	a.DailyEnd = strings.TrimSpace(a.DailyEnd)
	if a.Name == "" || len([]rune(a.Name)) > 128 {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "活动名称须为 1-128 个字符")
	}
	if a.Description == "" || len([]rune(a.Description)) > 512 {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "活动描述须为 1-512 个字符")
	}
	if len([]rune(a.Protocol)) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "协议不能超过 64 个字符")
	}
	if a.StartAtUnixMillis <= 0 || a.EndAtUnixMillis <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "活动时间不能为空")
	}
	if a.EndAtUnixMillis < a.StartAtUnixMillis {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "活动结束时间不能早于开始时间")
	}
	if a.DailyStart != "" && !activityDailyTimePattern.MatchString(a.DailyStart) {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "日时间范围开始格式须为 HH:mm:ss")
	}
	if a.DailyEnd != "" && !activityDailyTimePattern.MatchString(a.DailyEnd) {
		return nil, errorx.NewCodeError(errorx.CodeActivityInvalid, "日时间范围结束格式须为 HH:mm:ss")
	}
	row := repo.ActivityFromProto(a)
	if a.Id == 0 {
		baseKey := strings.TrimSpace(a.ActivityKey)
		if baseKey == "" {
			baseKey = a.Name
		}
		key, err := l.uniqueActivityKey(baseKey, 0)
		if err != nil {
			return nil, err
		}
		row.ActivityKey = key
		if err := repo.InsertActivity(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	} else {
		existing, err := repo.GetActivity(l.svcCtx.DB, int64(a.Id))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewCodeError(errorx.CodeActivityNotFound, "活动不存在")
			}
			return nil, err
		}
		row.ID = existing.ID
		row.ActivityKey = existing.ActivityKey
		if incomingKey := strings.TrimSpace(a.ActivityKey); incomingKey != "" && incomingKey != existing.ActivityKey {
			key, err := l.uniqueActivityKey(incomingKey, existing.ID)
			if err != nil {
				return nil, err
			}
			row.ActivityKey = key
		}
		if err := repo.UpdateActivity(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	}
	saved, err := repo.GetActivity(l.svcCtx.DB, row.ID)
	if err != nil {
		return nil, err
	}
	return saved.ToProto(), nil
}

func (l *UpsertActivityLogic) uniqueActivityKey(base string, excludeID int64) (string, error) {
	base = strings.TrimSpace(base)
	runes := []rune(base)
	if len(runes) > 64 {
		base = string(runes[:64])
	}
	key := base
	for i := 2; ; i++ {
		exists, err := repo.ActivityKeyExists(l.svcCtx.DB, key, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return key, nil
		}
		suffix := fmt.Sprintf("_%d", i)
		suffixRunes := []rune(suffix)
		baseRunes := []rune(base)
		if len(baseRunes)+len(suffixRunes) > 64 {
			baseRunes = baseRunes[:64-len(suffixRunes)]
		}
		key = string(baseRunes) + suffix
	}
}
