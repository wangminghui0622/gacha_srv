package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFullCarLogic {
	return &GetFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- M12 满车 full_car ----
func (l *GetFullCarLogic) GetFullCar(in *play.GetFullCarReq) (*play.GetFullCarResp, error) {
	return gameimpl.NewGetFullCarLogic(l.ctx, l.svcCtx).GetFullCar(in)
}
