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

	// 对外字符串 ID 以库中持久化的 public_id 为准；数字 ID 仍作内部主键。
	// 新令牌同时携带两者，不能再强制解码 public_id（否则密钥轮换后旧 ID 会失效）。
	userID := claims.UserId
	publicUserID := claims.PublicUserId
	if publicUserID == "" && userID > 0 {
		if u, err := l.svcCtx.UserModel.FindOne(l.ctx, userID); err == nil {
			publicUserID = publicUserIdOfUser(u)
		} else {
			publicUserID = publicid.MustEncodeUserID(userID)
		}
	}
	if userID <= 0 {
		if publicUserID == "" {
			return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "令牌缺少用户ID")
		}
		// 兼容只有 publicUserId 的旧令牌：先按当前密钥解码，失败再查库。
		decoded, err := publicid.DecodeUserID(publicUserID)
		if err != nil {
			if u, findErr := l.svcCtx.UserModel.FindOneByPublicId(l.ctx, publicUserID); findErr == nil {
				decoded = u.Id
			} else {
				return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "令牌用户ID无效")
			}
		}
		userID = decoded
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
