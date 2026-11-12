package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssignGashaponLuckyUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignGashaponLuckyUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignGashaponLuckyUserLogic {
	return &AssignGashaponLuckyUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockGashapon SAGA 03 正向：报价锁 + state FOR UPDATE + participants/records 追加 + 余票-count
func (l *AssignGashaponLuckyUserLogic) AssignGashaponLuckyUser(in *play.AssignGashaponLuckyUserReq) (*play.AssignGashaponLuckyUserResp, error) {
	return gameimpl.NewAssignGashaponLuckyUserLogic(l.ctx, l.svcCtx).AssignGashaponLuckyUser(in)
}
