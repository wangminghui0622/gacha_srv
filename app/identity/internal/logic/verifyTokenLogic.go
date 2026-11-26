package logic

import (
	"context"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/jwtutil"
	"gacha_srv/pkg/utils/publicid"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyTokenLogic {
	return &VerifyTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VerifyToken 网关 JwtAuth 中间件的鉴权回调：只验签名与有效期
func (l *VerifyTokenLogic) VerifyToken(in *identity.VerifyTokenReq) (*identity.VerifyTokenResp, error) {
	if in.Token == "" {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "令牌不能为空")
	}
	claims, err := jwtutil.ParseToken(l.svcCtx.Config.JwtAuth.AccessSecret, in.Token)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "令牌无效或已过期")
	}

	// 对外字符串 ID 是用户身份的主来源；数字 ID 仅作兼容与内部校验。
	userID := claims.UserId
	publicUserID := claims.PublicUserId
	if publicUserID != "" {
		decoded, err := publicid.DecodeUserID(publicUserID)
		if err != nil {
			return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "令牌用户ID无效")
		}
		if userID != 0 && userID != decoded {
			return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "令牌用户ID不一致")
		}
		userID = decoded
	} else if userID > 0 {
		publicUserID = publicid.MustEncodeUserID(userID)
	}
	if userID <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "令牌缺少用户ID")
	}
	return &identity.VerifyTokenResp{
		UserId:       userID,
		PublicUserId: publicUserID,
		Username:     claims.Username,
		Nickname:     claims.Nickname,
	}, nil
}
