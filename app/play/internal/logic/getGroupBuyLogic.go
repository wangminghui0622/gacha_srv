package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupBuyLogic {
	return &GetGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- M13 拼团抽 group_buy ----
func (l *GetGroupBuyLogic) GetGroupBuy(in *play.GetGroupBuyReq) (*play.GetGroupBuyResp, error) {
	return gameimpl.NewGetGroupBuyLogic(l.ctx, l.svcCtx).GetGroupBuy(in)
}
