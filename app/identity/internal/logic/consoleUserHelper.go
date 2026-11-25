package logic

import (
	"context"
	"errors"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"

	"gorm.io/gorm"
)

func buildConsoleUser(ctx context.Context, svcCtx *svc.ServiceContext, tenantID, userID int64) (*identity.ConsoleUser, error) {
	var user gacha_identity.UserUsers
	if err := svcCtx.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
		}
		return nil, err
	}
	profiles, err := repo.FetchUserProfiles(svcCtx.DB.WithContext(ctx), []int64{userID})
	if err != nil {
		return nil, err
	}
	profile, ok := profiles[userID]
	if !ok {
		profile = repo.UserProfile{
			UserID:       userID,
			Status:       1,
			LuckyDisplay: "初始化",
			RebateRate:   10,
			Identity:     "用户",
		}
	}
	if profile.Status == 0 {
		profile.Status = 1
	}
	if profile.LuckyDisplay == "" {
		profile.LuckyDisplay = "初始化"
	}
	if profile.Identity == "" {
		profile.Identity = "用户"
	}
	phone, err := svcCtx.PhoneRepo.Get(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	return &identity.ConsoleUser{
		UserId:              user.Id,
		Username:            user.Username,
		Nickname:            user.Nickname,
		Phone:               phone,
		AvatarUrl:           profile.AvatarURL,
		Status:              profile.Status,
		LuckyDisplay:        profile.LuckyDisplay,
		RegisterIp:          profile.RegisterIP,
		RegisterDevice:      profile.RegisterDevice,
		RebateRate:          profile.RebateRate,
		Identity:            profile.Identity,
		ParentUserId:        profile.ParentUserID,
		ReissueLuckPoints:   profile.ReissueLuckPoints == 1,
		CreatedAtUnixMillis: user.CreatedAt.UnixMilli(),
		UpdatedAtUnixMillis: user.UpdatedAt.UnixMilli(),
	}, nil
}
