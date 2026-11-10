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

type RequestPoolPublicationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// RequestPoolPublication 提请发布赏池（gacha.pool:request_publication；
// 申请人 = 会话主体 ActorID，不接受客户端传入）。
func NewRequestPoolPublicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestPoolPublicationLogic {
	return &RequestPoolPublicationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RequestPoolPublicationLogic) RequestPoolPublication(req *draw.RequestPoolPublicationReq) (*draw.PoolPublicationApproval, error) {
	claims := restx.ClaimsFrom(l.ctx)
	if claims == nil || claims.ActorID == 0 {
		return nil, errorx.NewCodeError(errorx.CodeConsoleTokenInvalid, "缺少会话主体")
	}
	req.RequesterId = int64(claims.ActorID)
	return l.svcCtx.DrawRpc.RequestPoolPublication(l.ctx, req)
}
