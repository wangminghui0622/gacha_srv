package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	"gacha_srv/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListDigBoxesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDigBoxesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigBoxesLogic {
	return &ListDigBoxesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListDigBoxes 挖宝箱列表（status=1 上架；非 SAGA RPC：业务错误 CodeError，系统错误 Internal）
func (l *ListDigBoxesLogic) ListDigBoxes(in *play.ListDigBoxesReq) (*play.ListDigBoxesResp, error) {
	var cfgs []gamemodel.DigBoxConfigs
	if err := l.svcCtx.DB.Where("status = ?", 1).Order("id ASC").Find(&cfgs).Error; err != nil {
		l.Errorf("ListDigBoxes query failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.DigBoxItem, 0, len(cfgs))
	for i := range cfgs {
		items = append(items, &play.DigBoxItem{
			BoxId:     cfgs[i].Id,
			Name:      cfgs[i].Name,
			Price:     utils.FenToYuan(cfgs[i].PriceFen),
			CellCount: cfgs[i].CellCount,
		})
	}
	return &play.ListDigBoxesResp{Items: items}, nil
}
