package identity
import (
	"context"

	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	identityClient "gacha_srv/app/identity/identityClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PhoneCodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPhoneCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PhoneCodeLogic {
	return &PhoneCodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// PhoneCode 手机号登录验证码：透传 identity 服务（下游 6002 频控/6003 未启用/6004 号码非法原样还原）
func (l *PhoneCodeLogic) PhoneCode(req *types.PhoneCodeReq) (resp *types.PhoneCodeResp, err error) {
	if req.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenant 非法（本仓单租户=1）")
	}
	r, err := l.svcCtx.IdentityRpc.RequestPhoneLoginCode(l.ctx, &identityClient.RequestPhoneLoginCodeReq{
		TenantId: req.TenantId,
		Phone:    req.Phone,
	})
	if err != nil {
		l.Errorf("RequestPhoneLoginCode failed, phone: %s, err: %v", req.Phone, err)
		return nil, errorx.FromError(err)
	}
	return &types.PhoneCodeResp{ExpiresAtUnix: r.ExpiresAtUnix}, nil
}
