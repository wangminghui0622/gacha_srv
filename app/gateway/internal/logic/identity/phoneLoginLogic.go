package identity
import (
	"context"

	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	identityClient "gacha_srv/app/identity/identityClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PhoneLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPhoneLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PhoneLoginLogic {
	return &PhoneLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// PhoneLogin 验证码登录：透传 identity 服务（6001 码错/1011 未绑定原样还原），签发 JWT
func (l *PhoneLoginLogic) PhoneLogin(req *types.PhoneLoginReq) (resp *types.LoginResp, err error) {
	if req.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenant 非法（本仓单租户=1）")
	}
	r, err := l.svcCtx.IdentityRpc.LoginWithPhoneCode(l.ctx, &identityClient.LoginWithPhoneCodeReq{
		TenantId:          req.TenantId,
		Phone:             req.Phone,
		Code:              req.Code,
		DeviceId:          req.DeviceId,
		ClientFingerprint: req.ClientFingerprint,
	})
	if err != nil {
		l.Errorf("LoginWithPhoneCode failed, phone: %s, err: %v", req.Phone, err)
		return nil, errorx.FromError(err)
	}
	return &types.LoginResp{UserId: r.UserId, Token: r.Token}, nil
}
