package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GrantGroupBuyPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantGroupBuyPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGroupBuyPrizesRevertLogic {
	return &GrantGroupBuyPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGroupBuyPrizesRevert SAGA 06 补偿：空操作（结算奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantGroupBuyPrizesRevertLogic) GrantGroupBuyPrizesRevert(in *play.GrantGroupBuyPrizesReq) (*play.GrantGroupBuyPrizesResp, error) {
	l.Infof("GrantGroupBuyPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantGroupBuyPrizesResp{Status: "GRANTED"}, nil
}
