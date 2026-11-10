package identity
import (
	"context"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	moneyClient "gacha_srv/app/money/moneyClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type RechargeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	// requestId 幂等键（HTTP Idempotency-Key 头，由 handler 注入，可选）
	requestId string
}

func NewRechargeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RechargeLogic {
	return &RechargeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetRequestId 由 handler 注入 HTTP Idempotency-Key 头（可选）
func (l *RechargeLogic) SetRequestId(id string) {
	l.requestId = id
}

// Recharge 充值（mock，直接加款）：userId 取 JWT 上下文；
// 幂等键优先取 Idempotency-Key 头（客户端重试带同一键只生效一次），否则网关生成 uuid
func (l *RechargeLogic) Recharge(req *types.RechargeReq) (resp *types.RechargeResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	requestId := l.requestId
	if requestId == "" {
		requestId = uuid.New().String()
	}
	r, err := l.svcCtx.MoneyRpc.Recharge(l.ctx, &moneyClient.RechargeReq{
		UserId:    userId,
		Amount:    req.Amount,
		RequestId: requestId,
	})
	if err != nil {
		l.Errorf("Recharge failed, userId: %d, err: %v", userId, err)
		return nil, errorx.FromError(err)
	}
	return &types.RechargeResp{Balance: r.Balance}, nil
}
