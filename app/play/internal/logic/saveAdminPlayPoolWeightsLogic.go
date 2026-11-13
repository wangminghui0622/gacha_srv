package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveAdminPlayPoolWeightsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveAdminPlayPoolWeightsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveAdminPlayPoolWeightsLogic {
	return &SaveAdminPlayPoolWeightsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *SaveAdminPlayPoolWeightsLogic) SaveAdminPlayPoolWeights(in *play.SaveAdminPlayPoolWeightsReq) (*play.SaveAdminPlayPoolWeightsResp, error) {
	return gameimpl.SaveAdminPlayPoolWeights(l.ctx, l.svcCtx, in)
}
