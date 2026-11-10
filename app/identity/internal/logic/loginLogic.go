package logic

import (
	"context"
	"errors"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/jwtutil"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Login 登录：校验密码，签发 JWT（网关凭 token 回调 VerifyToken 完成鉴权）
func (l *LoginLogic) Login(in *identity.LoginReq) (*identity.LoginResp, error) {
	if in.Username == "" || in.Password == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户名和密码不能为空")
	}
	u, err := l.svcCtx.UserModel.FindOneByUsername(l.ctx, in.Username)
	if err != nil {
		if errors.Is(err, gacha_identity.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
		}
		l.Errorf("Login find user failed, username: %s, err: %v", in.Username, err)
		return nil, errorx.FromMySQLError(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(in.Password)) != nil {
		return nil, errorx.NewCodeError(errorx.CodePasswordWrong, "密码错误")
	}
	token, err := jwtutil.NewToken(l.svcCtx.Config.JwtAuth.AccessSecret, l.svcCtx.Config.JwtAuth.AccessExpire,
		u.Id, u.Username, u.Nickname)
	if err != nil {
		l.Errorf("Login sign token failed, userId: %d, err: %v", u.Id, err)
		return nil, errorx.NewCodeError(errorx.CodeServerError, "令牌签发失败")
	}
	l.Infof("Login success, userId: %d", u.Id)
	return &identity.LoginResp{Token: token, UserId: u.Id}, nil
}
