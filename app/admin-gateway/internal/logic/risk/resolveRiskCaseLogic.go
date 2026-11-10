// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package risk

import (
	"context"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveRiskCaseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveRiskCaseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveRiskCaseLogic {
	return &ResolveRiskCaseLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveRiskCase 处置人 = 会话主体 ActorID（须与认领人一致，base-rpc 校验）；
// 同 operationId 同参重放由 base-rpc 幂等返回。
func (l *ResolveRiskCaseLogic) ResolveRiskCase(in *base.ResolveRiskCaseRequest) (*base.RiskCaseTransition, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil || claims.ActorID == 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "会话主体缺失")
	}
	in.ActorId = claims.ActorID
	in.TenantId = 1
	return l.svcCtx.BaseRpc.ResolveRiskCase(l.ctx, in)
}
