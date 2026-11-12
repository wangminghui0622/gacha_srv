package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckPrizesGrantedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckPrizesGrantedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckPrizesGrantedLogic {
	return &CheckPrizesGrantedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CheckPrizesGranted SAGA 04 补偿（CancelPayment）退款前置检查：
func (l *CheckPrizesGrantedLogic) CheckPrizesGranted(in *play.CheckPrizesGrantedReq) (*play.CheckPrizesGrantedResp, error) {
	return gameimpl.NewCheckPrizesGrantedLogic(l.ctx, l.svcCtx).CheckPrizesGranted(in)
}
