// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package eligibility

import (
	"context"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePolicyRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePolicyRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePolicyRevisionLogic {
	return &CreatePolicyRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreatePolicyRevision 透传 base-rpc（expectedLatestVersion CAS；同 hash 幂等）。
func (l *CreatePolicyRevisionLogic) CreatePolicyRevision(in *base.CreatePolicyRevisionRequest) (*base.EligibilityPolicyRecord, error) {
	return l.svcCtx.BaseRpc.CreatePolicyRevision(l.ctx, in)
}
