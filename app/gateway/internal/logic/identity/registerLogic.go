package identity
import (
	"context"

	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	identityClient "gacha_srv/app/identity/identityClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Register 注册：透传 identity 服务（用户名已存在等业务码由 ClientErrorInterceptor 还原）
func (l *RegisterLogic) Register(req *types.RegisterReq) (resp *types.RegisterResp, err error) {
	r, err := l.svcCtx.IdentityRpc.Register(l.ctx, &identityClient.RegisterReq{
		Username: req.Username,
		Password: req.Password,
		Nickname: req.Nickname,
	})
	if err != nil {
		l.Errorf("Register failed, username: %s, err: %v", req.Username, err)
		return nil, errorx.FromError(err)
	}
	return &types.RegisterResp{UserId: r.UserId}, nil
}
