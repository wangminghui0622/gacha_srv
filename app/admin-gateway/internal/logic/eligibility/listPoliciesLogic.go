// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package eligibility

import (
	"context"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPoliciesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPoliciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPoliciesLogic {
	return &ListPoliciesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPolicies 透传 base-rpc（游标分页）。
func (l *ListPoliciesLogic) ListPolicies(in *base.ListPoliciesRequest) (*base.ListPoliciesReply, error) {
	return l.svcCtx.BaseRpc.ListPolicies(l.ctx, in)
}
