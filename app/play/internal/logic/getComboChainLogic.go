package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetComboChainLogic {
	return &GetComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- M13 连击链 combo_chain ----
func (l *GetComboChainLogic) GetComboChain(in *play.GetComboChainReq) (*play.GetComboChainResp, error) {
	return gameimpl.NewGetComboChainLogic(l.ctx, l.svcCtx).GetComboChain(in)
}
