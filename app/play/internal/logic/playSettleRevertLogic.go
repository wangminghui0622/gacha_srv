package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlaySettleRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlaySettleRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlaySettleRevertLogic {
	return &PlaySettleRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PlaySettleRevertLogic) PlaySettleRevert(in *play.PlaySettleReq) (*play.PlaySettleResp, error) {
	return doPlaySettle(l.ctx, l.svcCtx, in, true)
}
