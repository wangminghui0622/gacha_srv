package logic

import (
	"context"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/jwtutil"

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
	return &identity.VerifyTokenResp{
		UserId:   claims.UserId,
		Username: claims.Username,
		Nickname: claims.Nickname,
	}, nil
}
