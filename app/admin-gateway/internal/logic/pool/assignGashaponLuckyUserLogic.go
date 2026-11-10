// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/play/play"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AssignGashaponLuckyUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// AssignGashaponLuckyUser 指定幸运用户（gacha.pool:assign_lucky；
// 扭蛋 LUCKY 奖指派，userId=0 清除，透传 game-rpc）。
func NewAssignGashaponLuckyUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignGashaponLuckyUserLogic {
	return &AssignGashaponLuckyUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AssignGashaponLuckyUserLogic) AssignGashaponLuckyUser(req *play.AssignGashaponLuckyUserReq) (*play.AssignGashaponLuckyUserResp, error) {
	return l.svcCtx.PlayRpc.AssignGashaponLuckyUser(l.ctx, req)
}
