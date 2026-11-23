package logic

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/phonecrypto"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// testPhoneLoginCode 测试阶段固定验证码；手机号首次登录时自动建号并绑定。
const testPhoneLoginCode = "888888"

type LoginWithPhoneCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginWithPhoneCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginWithPhoneCodeLogic {
	return &LoginWithPhoneCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LoginWithPhoneCode 验证码登录：当前阶段固定验证码 888888。
// 手机号已绑定则直接登录；未绑定则自动创建账号并绑定手机号。
// 非固定验证码仍走原有一次校验码流程（错码/频控原样透出）。
func (l *LoginWithPhoneCodeLogic) LoginWithPhoneCode(in *identity.LoginWithPhoneCodeReq) (*identity.LoginResp, error) {
	phone, err := phonecrypto.NormalizeCNMobile(in.Phone)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "手机号格式不正确")
	}
	if in.Code != testPhoneLoginCode {
		if _, err := l.svcCtx.ThirdpartyRpc.CheckCaptcha(l.ctx, &thirdparty.CheckCaptchaReq{
			TenantId: in.TenantId,
			Phone:    phone,
			Code:     in.Code,
			Scene:    thirdparty.CaptchaScene_CAPTCHA_SCENE_LOGIN,
		}); err != nil {
			return nil, err
		}
	} else {
		l.Infof("Phone login using fixed test code, phone: %s", maskPhone(phone))
	}

	userID, err := l.svcCtx.PhoneRepo.FindExact(l.ctx, in.TenantId, phone)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		userID, err = l.createPhoneUser(in.TenantId, phone)
	}
	if err != nil || userID == 0 {
		l.Errorf("Phone login find user failed, phone: %s, err: %v", maskPhone(phone), err)
		return nil, errorx.NewCodeError(errorx.CodePhoneLoginInvalid, "手机号未绑定或验证失败")
	}
	return issueToken(l.ctx, l.svcCtx, userID)
}

// createPhoneUser 测试登录兜底：为未绑定的手机号自动创建账号并绑定。
func (l *LoginWithPhoneCodeLogic) createPhoneUser(tenantID int64, phone string) (int64, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return 0, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(buf)), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	user := &gacha_identity.UserUsers{
		Username:  l.phoneUsername(phone),
		Password:  string(hash),
		Nickname:  maskPhone(phone),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := l.svcCtx.UserModel.Insert(l.ctx, nil, user); err != nil {
		// 并发首登可能已创建并绑定成功，回查一次。
		if uid, findErr := l.svcCtx.PhoneRepo.FindExact(l.ctx, tenantID, phone); findErr == nil && uid > 0 {
			return uid, nil
		}
		return 0, err
	}
	if err := l.svcCtx.PhoneRepo.Set(l.ctx, tenantID, user.Id, phone); err != nil {
		_ = l.svcCtx.UserModel.Delete(l.ctx, nil, user.Id)
		if uid, findErr := l.svcCtx.PhoneRepo.FindExact(l.ctx, tenantID, phone); findErr == nil && uid > 0 {
			return uid, nil
		}
		return 0, err
	}
	l.Infof("Phone login auto-created user, userId: %d, phone: %s", user.Id, maskPhone(phone))
	return user.Id, nil
}

func (l *LoginWithPhoneCodeLogic) phoneUsername(phone string) string {
	base := "phone_" + phone
	for i := 1; i <= 20; i++ {
		username := base
		if i > 1 {
			username = fmt.Sprintf("%s_%d", base, i)
		}
		if _, err := l.svcCtx.UserModel.FindOneByUsername(l.ctx, username); errors.Is(err, gacha_identity.ErrNotFound) {
			return username
		}
	}
	return fmt.Sprintf("phone_%s_%d", phone, time.Now().UnixNano())
}

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}
