// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package eligibility

import (
	"context"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePolicyLogic {
	return &CreatePolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreatePolicy 透传 base-rpc（创建 DRAFT v1）。
func (l *CreatePolicyLogic) CreatePolicy(in *base.CreatePolicyDraftRequest) (*base.EligibilityPolicyRecord, error) {
	return l.svcCtx.BaseRpc.CreatePolicyDraft(l.ctx, in)
}
