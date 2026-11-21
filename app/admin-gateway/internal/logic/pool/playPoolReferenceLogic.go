package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryPlayPoolReferencesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewQueryPlayPoolReferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryPlayPoolReferencesLogic {
	return &QueryPlayPoolReferencesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *QueryPlayPoolReferencesLogic) QueryPlayPoolReferences(req *play.QueryPlayPoolReferencesReq) (*play.QueryPlayPoolReferencesResp, error) {
	return l.svcCtx.PlayRpc.QueryPlayPoolReferences(l.ctx, req)
}
