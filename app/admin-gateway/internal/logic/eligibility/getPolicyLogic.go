// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package eligibility

import (
	"context"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPolicyLogic {
	return &GetPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetPolicy 透传 base-rpc（version 0=当前版本）。
func (l *GetPolicyLogic) GetPolicy(in *base.GetPolicyRequest) (*base.EligibilityPolicyRecord, error) {
	return l.svcCtx.BaseRpc.GetPolicy(l.ctx, in)
}
