package logic

import (
	"context"
	"errors"

	"gacha_srv/app/identity/internal/phonecrypto"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/jwtutil"

	"gorm.io/gorm"
)

// issueToken 登录成功签发本仓单 JWT（D3：会话维持本仓设计，响应 {userId, token}）
func issueToken(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) (*identity.LoginResp, error) {
	u, err := svcCtx.UserModel.FindOne(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
		}
		return nil, errorx.FromMySQLError(err)
	}
	token, err := jwtutil.NewToken(svcCtx.Config.JwtAuth.AccessSecret, svcCtx.Config.JwtAuth.AccessExpire,
		u.Id, u.Username, u.Nickname)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeServerError, "令牌签发失败")
	}
	return &identity.LoginResp{UserId: u.Id, Token: token}, nil
}

// wechatKindFromScene 微信授权场景 → 第三方种类（identity.proto 两枚举一一对应）
func wechatKindFromScene(scene identity.WeChatOAuthScene) (int64, error) {
	switch scene {
	case identity.WeChatOAuthScene_WECHAT_OAUTH_SCENE_MINI_PROGRAM:
		return int64(identity.ThirdPartyKind_THIRD_PARTY_KIND_WECHAT_MINI_PROGRAM), nil
	case identity.WeChatOAuthScene_WECHAT_OAUTH_SCENE_APP:
		return int64(identity.ThirdPartyKind_THIRD_PARTY_KIND_WECHAT_APP), nil
	default:
		return 0, errorx.NewCodeError(errorx.CodeParamError, "微信授权场景非法")
	}
}

func validThirdPartyKind(kind identity.ThirdPartyKind) bool {
	return kind == identity.ThirdPartyKind_THIRD_PARTY_KIND_WECHAT_MINI_PROGRAM ||
		kind == identity.ThirdPartyKind_THIRD_PARTY_KIND_WECHAT_APP
}

// mapPhoneRepoError 手机号仓库错误 → errorx（server_v2 phoneError 映射语义）
func mapPhoneRepoError(err error) error {
	switch {
	case errors.Is(err, phonecrypto.ErrInvalidPhone):
		return errorx.NewCodeError(errorx.CodeParamError, "手机号或搜索片段非法")
	case errors.Is(err, repo.ErrPhoneInUse):
		return errorx.NewCodeError(errorx.CodePhoneInUse, "手机号已被其他用户绑定")
	case errors.Is(err, repo.ErrPhoneSearchTooBroad):
		return errorx.NewCodeError(errorx.CodePhoneSearchTooBroad, "手机号片段搜索范围过大")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return errorx.NewCodeError(errorx.CodeNotFound, "手机号未绑定")
	default:
		return errorx.FromMySQLError(err)
	}
}

// mapThirdPartyRepoError 第三方身份仓库错误 → errorx
func mapThirdPartyRepoError(err error) error {
	if errors.Is(err, repo.ErrThirdPartyNotLinked) {
		return errorx.NewCodeError(errorx.CodeThirdPartyNotLinked, "第三方身份未绑定")
	}
	return errorx.FromMySQLError(err)
}
