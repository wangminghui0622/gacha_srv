package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryPlayPoolReferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryPlayPoolReferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryPlayPoolReferencesLogic {
	return &QueryPlayPoolReferencesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *QueryPlayPoolReferencesLogic) QueryPlayPoolReferences(in *play.QueryPlayPoolReferencesReq) (*play.QueryPlayPoolReferencesResp, error) {
	return gameimpl.NewQueryPlayPoolReferencesLogic(l.ctx, l.svcCtx).QueryPlayPoolReferences(in)
}
