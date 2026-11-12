package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlaySettleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlaySettleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlaySettleLogic {
	return &PlaySettleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PlaySettleLogic) PlaySettle(in *play.PlaySettleReq) (*play.PlaySettleResp, error) {
	return doPlaySettle(l.ctx, l.svcCtx, in, false)
}
