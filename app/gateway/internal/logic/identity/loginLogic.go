package identity
import (
	"context"

	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	identityClient "gacha_srv/app/identity/identityClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Login 登录：透传 identity 服务，签发 JWT（网关后续请求带 Authorization: Bearer <token>）
func (l *LoginLogic) Login(req *types.LoginReq) (resp *types.LoginResp, err error) {
	r, err := l.svcCtx.IdentityRpc.Login(l.ctx, &identityClient.LoginReq{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		l.Errorf("Login failed, username: %s, err: %v", req.Username, err)
		return nil, errorx.FromError(err)
	}
	return &types.LoginResp{UserId: r.UserId, Token: r.Token}, nil
}
