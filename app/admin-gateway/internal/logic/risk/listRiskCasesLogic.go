// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package risk

import (
	"context"

	"gacha_srv/app/base/base"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRiskCasesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRiskCasesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRiskCasesLogic {
	return &ListRiskCasesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListRiskCases 透传 base-rpc（游标分页；actorId 0=全部）。
func (l *ListRiskCasesLogic) ListRiskCases(in *base.ListRiskCasesRequest) (*base.ListRiskCasesReply, error) {
	return l.svcCtx.BaseRpc.ListRiskCases(l.ctx, in)
}
