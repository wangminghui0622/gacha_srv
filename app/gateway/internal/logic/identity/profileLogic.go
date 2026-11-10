package identity
import (
	"context"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	identityClient "gacha_srv/app/identity/identityClient"
	moneyClient "gacha_srv/app/money/moneyClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileLogic {
	return &ProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Profile 用户资料（含余额）：userId 取 JWT 上下文，不信客户端；
// 资料走 identity-rpc，余额走 money-rpc（旧仓同走 identity-rpc，拆分后见 service-division.md §2.1）
func (l *ProfileLogic) Profile() (resp *types.UserProfile, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	p, err := l.svcCtx.IdentityRpc.GetProfile(l.ctx, &identityClient.GetProfileReq{UserId: userId})
	if err != nil {
		l.Errorf("Profile GetProfile failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	b, err := l.svcCtx.MoneyRpc.GetBalance(l.ctx, &moneyClient.GetBalanceReq{UserId: userId})
	if err != nil {
		l.Errorf("Profile GetBalance failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	return &types.UserProfile{
		UserId:   p.UserId,
		Username: p.Username,
		Nickname: p.Nickname,
		Balance:  b.Balance,
	}, nil
}
