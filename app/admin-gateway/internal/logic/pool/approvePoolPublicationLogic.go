// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/restx"
	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApprovePoolPublicationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// ApprovePoolPublication 审批赏池发布（gacha.pool:approve_publication；
// 审批人 = 会话主体 ActorID，≠ 申请人，由 draw-rpc 校验）。
func NewApprovePoolPublicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovePoolPublicationLogic {
	return &ApprovePoolPublicationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovePoolPublicationLogic) ApprovePoolPublication(req *draw.ApprovePoolPublicationReq) (*draw.PoolPublicationApproval, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil || claims.ActorID == 0 {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话主体")
	}
	req.ApproverId = int64(claims.ActorID)
	return l.svcCtx.DrawRpc.ApprovePoolPublication(l.ctx, req)
}
