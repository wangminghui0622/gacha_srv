package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveAdminPlayPoolWeightsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveAdminPlayPoolWeightsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveAdminPlayPoolWeightsLogic {
	return &SaveAdminPlayPoolWeightsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SaveAdminPlayPoolWeightsLogic) SaveAdminPlayPoolWeights(req *play.SaveAdminPlayPoolWeightsReq) (*play.SaveAdminPlayPoolWeightsResp, error) {
	return l.svcCtx.PlayRpc.SaveAdminPlayPoolWeights(l.ctx, req)
}
