package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGashaponLogic {
	return &GetGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- M12 扭蛋 gashapon ----
func (l *GetGashaponLogic) GetGashapon(in *play.GetGashaponReq) (*play.GetGashaponResp, error) {
	return gameimpl.NewGetGashaponLogic(l.ctx, l.svcCtx).GetGashapon(in)
}
