package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDigBoxSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDigBoxSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDigBoxSessionLogic {
	return &GetDigBoxSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDigBoxSessionLogic) GetDigBoxSession(in *play.GetDigBoxSessionReq) (*play.GetDigBoxSessionResp, error) {
	return gameimpl.NewGetDigBoxSessionLogic(l.ctx, l.svcCtx).GetDigBoxSession(in)
}
