package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type UpdateConsoleUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateConsoleUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateConsoleUserLogic {
	return &UpdateConsoleUserLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateConsoleUserLogic) UpdateConsoleUser(in *identity.UpdateConsoleUserReq) (*identity.ConsoleUser, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenantId 须为 1（单租户）")
	}
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID不能为空")
	}
	nickname := strings.TrimSpace(in.Nickname)
	if nickname == "" || len([]rune(nickname)) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "昵称须为 1-32 个字符")
	}
	avatarURL := strings.TrimSpace(in.AvatarUrl)
	if avatarURL == "" || len(avatarURL) > 512 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请上传头像")
	}
	status := in.Status
	if status == 0 {
		status = 1
	}
	if status != 1 && status != 2 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "状态值无效")
	}
	luckyDisplay := strings.TrimSpace(in.LuckyDisplay)
	if luckyDisplay == "" {
		luckyDisplay = "初始化"
	}
	if len([]rune(luckyDisplay)) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "集市展示状态不能超过 32 个字符")
	}
	identityName := strings.TrimSpace(in.Identity)
	if identityName == "" {
		identityName = "用户"
	}
	if len([]rune(identityName)) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "身份不能超过 32 个字符")
	}
	if in.RebateRate < 0 || in.RebateRate > 100 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "返佣比例须为 0-100")
	}
	if in.ParentUserId < 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "上级用户ID无效")
	}
	if in.ParentUserId == in.UserId {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "上级用户不能是自己")
	}
	if in.ParentUserId > 0 {
		if _, err := l.svcCtx.UserModel.FindOne(l.ctx, in.ParentUserId); err != nil {
			if errors.Is(err, gacha_identity.ErrNotFound) {
				return nil, errorx.NewCodeError(errorx.CodeParamError, "上级用户不存在")
			}
			return nil, err
		}
	}

	var user gacha_identity.UserUsers
	if err := l.svcCtx.DB.WithContext(l.ctx).Where("id = ?", in.UserId).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
		}
		return nil, err
	}
	if err := l.svcCtx.DB.WithContext(l.ctx).Model(&gacha_identity.UserUsers{}).
		Where("id = ?", in.UserId).Update("nickname", nickname).Error; err != nil {
		return nil, err
	}

	profiles, err := repo.FetchUserProfiles(l.svcCtx.DB.WithContext(l.ctx), []int64{in.UserId})
	if err != nil {
		return nil, err
	}
	profile, ok := profiles[in.UserId]
	if !ok {
		profile = repo.UserProfile{
			UserID:    in.UserId,
			CreatedAt: time.Now().UTC(),
		}
	}
	profile.AvatarURL = avatarURL
	profile.Status = status
	profile.LuckyDisplay = luckyDisplay
	profile.RebateRate = in.RebateRate
	profile.Identity = identityName
	profile.ParentUserID = in.ParentUserId
	if in.ReissueLuckPoints {
		profile.ReissueLuckPoints = 1
	} else {
		profile.ReissueLuckPoints = 0
	}
	profile.UpdatedAt = time.Now().UTC()
	if err := repo.SaveUserProfile(l.svcCtx.DB.WithContext(l.ctx), &profile); err != nil {
		return nil, err
	}
	return buildConsoleUser(l.ctx, l.svcCtx, in.TenantId, in.UserId)
}
