package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GrantComboChainPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantComboChainPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantComboChainPrizesRevertLogic {
	return &GrantComboChainPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantComboChainPrizesRevert SAGA 06 补偿：空操作（结算奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantComboChainPrizesRevertLogic) GrantComboChainPrizesRevert(in *play.GrantComboChainPrizesReq) (*play.GrantComboChainPrizesResp, error) {
	l.Infof("GrantComboChainPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantComboChainPrizesResp{Status: "GRANTED"}, nil
}
