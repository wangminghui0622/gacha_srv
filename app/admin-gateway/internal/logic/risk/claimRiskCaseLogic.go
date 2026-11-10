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

type ClaimRiskCaseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimRiskCaseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimRiskCaseLogic {
	return &ClaimRiskCaseLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClaimRiskCase 认领人 = 会话主体 ActorID（不接受客户端传入——照抄 server_v2）；
// 同 operationId 同参重放由 base-rpc 幂等返回。
func (l *ClaimRiskCaseLogic) ClaimRiskCase(in *base.ClaimRiskCaseRequest) (*base.RiskCaseTransition, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil || claims.ActorID == 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "会话主体缺失")
	}
	in.ActorId = claims.ActorID
	in.TenantId = 1
	return l.svcCtx.BaseRpc.ClaimRiskCase(l.ctx, in)
}
