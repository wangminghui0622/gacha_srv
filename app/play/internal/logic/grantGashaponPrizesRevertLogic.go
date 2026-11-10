package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GrantGashaponPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantGashaponPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGashaponPrizesRevertLogic {
	return &GrantGashaponPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGashaponPrizesRevert SAGA 06 补偿：空操作（特别奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantGashaponPrizesRevertLogic) GrantGashaponPrizesRevert(in *play.GrantGashaponPrizesReq) (*play.GrantGashaponPrizesResp, error) {
	l.Infof("GrantGashaponPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantGashaponPrizesResp{Status: "GRANTED"}, nil
}
