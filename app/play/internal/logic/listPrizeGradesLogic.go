package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPrizeGradesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPrizeGradesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPrizeGradesLogic {
	return &ListPrizeGradesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPrizeGrades 赏级配置（赏别列表，控制台只读）。
func (l *ListPrizeGradesLogic) ListPrizeGrades(in *play.ListPrizeGradesReq) (*play.ListPrizeGradesResp, error) {
	rows, err := l.svcCtx.PrizeGrades.List(l.ctx)
	if err != nil {
		return nil, err
	}
	out := &play.ListPrizeGradesResp{Items: make([]*play.PrizeGrade, 0, len(rows))}
	for _, row := range rows {
		out.Items = append(out.Items, &play.PrizeGrade{Sort: row.Sort, Code: row.Code, Category: row.Category})
	}
	return out, nil
}
