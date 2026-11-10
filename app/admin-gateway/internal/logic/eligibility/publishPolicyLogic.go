// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package eligibility

import (
	"context"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishPolicyLogic {
	return &PublishPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishPolicy 透传 base-rpc（expectedConfigHash CAS；旧 PUBLISHED 自动退役）。
func (l *PublishPolicyLogic) PublishPolicy(in *base.PublishPolicyRequest) (*base.EligibilityPolicyRecord, error) {
	return l.svcCtx.BaseRpc.PublishPolicy(l.ctx, in)
}
