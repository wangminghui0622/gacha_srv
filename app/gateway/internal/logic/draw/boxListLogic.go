package draw

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BoxListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBoxListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BoxListLogic {
	return &BoxListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// BoxList 宝箱列表：透传 draw 服务（售罄状态含在返回项）
func (l *BoxListLogic) BoxList() (resp *types.BoxListResp, err error) {
	r, err := l.svcCtx.DrawRpc.ListBoxes(l.ctx, &drawClient.ListBoxesReq{})
	if err != nil {
		l.Errorf("BoxList failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	boxes := make([]types.BoxItem, 0, len(r.Boxes))
	for _, b := range r.Boxes {
		boxes = append(boxes, types.BoxItem{
			BoxId:       b.BoxId,
			BoxName:     b.BoxName,
			Price:       b.Price,
			Description: b.Description,
			SoldOut:     b.SoldOut,
		})
	}
	return &types.BoxListResp{List: boxes}, nil
}
