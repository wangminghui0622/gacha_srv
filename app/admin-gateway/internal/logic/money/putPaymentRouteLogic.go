// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package money

import (
	"context"

	"gacha_srv/app/money/money"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PutPaymentRouteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPutPaymentRouteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutPaymentRouteLogic {
	return &PutPaymentRouteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PutPaymentRoute 操作员 = 会话主体 ActorID（不接受客户端传入——照抄 server_v2）；
// tenant 固定 1（policy.tenantId 由网关注入）；expectedVersion CAS 冲突 → 8004。
func (l *PutPaymentRouteLogic) PutPaymentRoute(in *money.PutPaymentRouteRequest) (*money.PutPaymentRouteReply, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil || claims.ActorID == 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "会话主体缺失")
	}
	in.OperatorUserId = claims.ActorID
	if in.Policy != nil {
		in.Policy.TenantId = 1
		// 行键 (tenant,channel,scene) 存在 policy 行内——网关注入顶层 channel/scene（照抄 server_v2 顶层携带语义）
		in.Policy.Channel = in.Channel
		in.Policy.Scene = in.Scene
	}
	return l.svcCtx.MoneyRpc.PutPaymentRoute(l.ctx, in)
}
