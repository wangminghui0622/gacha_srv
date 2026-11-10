package draw

import (
	"context"

	drawClient "gacha_srv/app/draw/drawClient"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BoxDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBoxDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BoxDetailLogic {
	return &BoxDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// BoxDetail 宝箱详情（概率公示）：透传 draw 服务
func (l *BoxDetailLogic) BoxDetail(req *types.BoxDetailPathReq) (resp *types.BoxDetailResp, err error) {
	r, err := l.svcCtx.DrawRpc.BoxDetail(l.ctx, &drawClient.BoxDetailReq{BoxId: req.BoxId})
	if err != nil {
		l.Errorf("BoxDetail failed, boxId: %d, err: %v", req.BoxId, err)
		return nil, errorx.FromError(err)
	}
	entries := make([]types.PoolEntryItem, 0, len(r.Entries))
	for _, e := range r.Entries {
		entries = append(entries, types.PoolEntryItem{
			EntryId:     e.EntryId,
			Name:        e.Name,
			Weight:      e.Weight,
			Remaining:   e.Remaining,
			IsPrize:     e.IsPrize,
			Probability: e.Probability,
		})
	}
	return &types.BoxDetailResp{
		BoxId:       r.BoxId,
		BoxName:     r.BoxName,
		Price:       r.Price,
		Description: r.Description,
		SoldOut:     r.SoldOut,
		Entries:     entries,
	}, nil
}
