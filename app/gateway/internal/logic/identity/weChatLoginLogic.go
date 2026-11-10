package identity

import (
	"context"

	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	userPb "gacha_srv/app/identity/identity"
	identityClient "gacha_srv/app/identity/identityClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type WeChatLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWeChatLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WeChatLoginLogic {
	return &WeChatLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WeChatLogin 微信登录：透传 user 服务（6005 未配置/6006 微信接口失败/1013 未绑定原样还原），签发 JWT
func (l *WeChatLoginLogic) WeChatLogin(req *types.WeChatLoginReq) (resp *types.LoginResp, err error) {
	if req.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenant 非法（本仓单租户=1）")
	}
	if req.Scene != 1 && req.Scene != 2 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "scene 非法（1=App 2=小程序）")
	}
	r, err := l.svcCtx.IdentityRpc.LoginWithWeChatCode(l.ctx, &identityClient.LoginWithWeChatCodeReq{
		TenantId:          req.TenantId,
		Code:              req.Code,
		Scene:             userPb.WeChatOAuthScene(req.Scene),
		DeviceId:          req.DeviceId,
		ClientFingerprint: req.ClientFingerprint,
	})
	if err != nil {
		l.Errorf("LoginWithWeChatCode failed, scene: %d, err: %v", req.Scene, err)
		return nil, errorx.FromError(err)
	}
	return &types.LoginResp{UserId: r.UserId, Token: r.Token}, nil
}
