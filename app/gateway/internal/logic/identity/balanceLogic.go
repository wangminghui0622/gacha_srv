package identity
import (
	"context"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	moneyClient "gacha_srv/app/money/moneyClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BalanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BalanceLogic {
	return &BalanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Balance 余额查询：userId 取 JWT 上下文，不信客户端
func (l *BalanceLogic) Balance() (resp *types.BalanceResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.MoneyRpc.GetBalance(l.ctx, &moneyClient.GetBalanceReq{UserId: userId})
	if err != nil {
		l.Errorf("Balance failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	return &types.BalanceResp{Balance: r.Balance}, nil
}
