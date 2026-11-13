package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPrizeGradesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPrizeGradesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPrizeGradesLogic {
	return &ListPrizeGradesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListPrizeGradesLogic) ListPrizeGrades(req *play.ListPrizeGradesReq) (*play.ListPrizeGradesResp, error) {
	return l.svcCtx.PlayRpc.ListPrizeGrades(l.ctx, req)
}
