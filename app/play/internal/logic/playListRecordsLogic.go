package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlayListRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlayListRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlayListRecordsLogic {
	return &PlayListRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PlayListRecordsLogic) PlayListRecords(in *play.PlayListRecordsReq) (*play.PlayListRecordsResp, error) {
	return doPlayListRecords(l.ctx, l.svcCtx, in)
}
