package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/phonecrypto"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type CreateConsoleUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateConsoleUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateConsoleUserLogic {
	return &CreateConsoleUserLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// CreateConsoleUser 后台创建用户（昵称/手机号/头像/返佣比例/身份）。
func (l *CreateConsoleUserLogic) CreateConsoleUser(in *identity.CreateConsoleUserReq) (*identity.ConsoleUser, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenantId 须为 1（单租户）")
	}
	nickname := strings.TrimSpace(in.Nickname)
	if nickname == "" || len([]rune(nickname)) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "昵称须为 1-32 个字符")
	}
	avatarURL := strings.TrimSpace(in.AvatarUrl)
	if avatarURL == "" || len(avatarURL) > 512 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请上传头像")
	}
	if in.RebateRate < 0 || in.RebateRate > 100 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "返佣比例须为 0-100")
	}
	identityName := strings.TrimSpace(in.Identity)
	if identityName == "" {
		identityName = "用户"
	}
	if len([]rune(identityName)) > 32 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "身份不能超过 32 个字符")
	}
	var phone string
	if rawPhone := strings.TrimSpace(in.Phone); rawPhone != "" {
		normalized, err := phonecrypto.NormalizeCNMobile(rawPhone)
		if err != nil {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "手机号格式不正确")
		}
		phone = normalized
	}

	username, err := l.generateUsername()
	if err != nil {
		return nil, err
	}
	randomPassword, err := randomToken(16)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeServerError, "生成用户密码失败")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeServerError, "密码加密失败")
	}

	now := time.Now().UTC()
	user := &gacha_identity.UserUsers{
		Username:  username,
		Password:  string(hash),
		Nickname:  nickname,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := l.svcCtx.UserModel.Insert(l.ctx, nil, user); err != nil {
		return nil, errorx.FromMySQLError(err)
	}

	profile := &repo.UserProfile{
		UserID:       user.Id,
		AvatarURL:    avatarURL,
		Status:       1,
		LuckyDisplay: "初始化",
		RebateRate:   in.RebateRate,
		Identity:     identityName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := repo.SaveUserProfile(l.svcCtx.DB.WithContext(l.ctx), profile); err != nil {
		_ = l.svcCtx.UserModel.Delete(l.ctx, nil, user.Id)
		return nil, err
	}
	if phone != "" {
		if err := l.svcCtx.PhoneRepo.Set(l.ctx, in.TenantId, user.Id, phone); err != nil {
			_ = l.svcCtx.DB.WithContext(l.ctx).Where("user_id = ?", user.Id).Delete(&repo.UserProfile{}).Error
			_ = l.svcCtx.UserModel.Delete(l.ctx, nil, user.Id)
			if err == repo.ErrPhoneInUse {
				return nil, errorx.NewCodeError(errorx.CodeParamError, "手机号已被其他用户绑定")
			}
			return nil, err
		}
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
		CreatedAtUnixMillis: user.CreatedAt.UnixMilli(),
		UpdatedAtUnixMillis: user.UpdatedAt.UnixMilli(),
	}, nil
}

func (l *CreateConsoleUserLogic) generateUsername() (string, error) {
	for i := 0; i < 5; i++ {
		token, err := randomToken(4)
		if err != nil {
			return "", errorx.NewCodeError(errorx.CodeServerError, "生成用户名失败")
		}
		username := fmt.Sprintf("u_%s", token)
		_, err = l.svcCtx.UserModel.FindOneByUsername(l.ctx, username)
		if err == nil {
			continue
		}
		if err == gacha_identity.ErrNotFound {
			return username, nil
		}
		return "", err
	}
	return "", errorx.NewCodeError(errorx.CodeServerError, "生成用户名失败")
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
