package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GrantFullCarPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantFullCarPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantFullCarPrizesRevertLogic {
	return &GrantFullCarPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantFullCarPrizesRevert SAGA 06 补偿：空操作（整车奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantFullCarPrizesRevertLogic) GrantFullCarPrizesRevert(in *play.GrantFullCarPrizesReq) (*play.GrantFullCarPrizesResp, error) {
	l.Infof("GrantFullCarPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantFullCarPrizesResp{Status: "GRANTED"}, nil
}
