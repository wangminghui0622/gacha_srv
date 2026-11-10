package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GrantDigPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantDigPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantDigPrizesRevertLogic {
	return &GrantDigPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantDigPrizesRevert SAGA 06 补偿：空操作（D4 决策——奖品先于捕获且不追回，
// 补偿注册仅为防御；即使调用也幂等返回）
func (l *GrantDigPrizesRevertLogic) GrantDigPrizesRevert(in *play.GrantDigPrizesReq) (*play.GrantDigPrizesResp, error) {
	l.Infof("GrantDigPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantDigPrizesResp{Status: "GRANTED"}, nil
}
