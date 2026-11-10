package logic

import (
	"context"
	"errors"
	"fmt"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/svc"
	drawmodel "gacha_srv/pkg/model/gacha_draw"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BoxDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBoxDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BoxDetailLogic {
	return &BoxDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BoxDetail 宝箱详情（概率公示）：
// 宝箱 = 缓存主键读（10 分钟 TTL）；条目 = 缓存列表读（3 分钟短 TTL，
// 写路径「事务内删键 + commit 后延迟双删」保证读到的 remaining 基本 fresh）。
func (l *BoxDetailLogic) BoxDetail(in *draw.BoxDetailReq) (*draw.BoxDetailResp, error) {
	if in.BoxId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "宝箱ID无效")
	}
	b, err := l.svcCtx.BoxModel.FindOne(l.ctx, in.BoxId)
	if err != nil {
		if errors.Is(err, drawmodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeBoxNotFound, "宝箱不存在")
		}
		l.Errorf("BoxDetail find box failed, boxId: %d, err: %v", in.BoxId, err)
		return nil, errorx.FromMySQLError(err)
	}
	entries, err := l.svcCtx.EntryModel.FindAllByBoxIdCached(l.ctx, in.BoxId)
	if err != nil {
		l.Errorf("BoxDetail find entries failed, boxId: %d, err: %v", in.BoxId, err)
		return nil, errorx.FromMySQLError(err)
	}

	// 公示概率 = weight / 同箱 Σweight（分母含售罄条目，公示口径稳定不随库存漂移）
	totalWeight := int64(0)
	prizeDrawable := false
	for _, e := range entries {
		totalWeight += e.Weight
		if e.IsPrize == 1 && (e.Remaining > 0 || e.Quantity == -1) {
			prizeDrawable = true
		}
	}
	items := make([]*draw.PoolEntryItem, 0, len(entries))
	for _, e := range entries {
		p := 0.0
		if totalWeight > 0 {
			p = float64(e.Weight) / float64(totalWeight) * 100
		}
		items = append(items, &draw.PoolEntryItem{
			EntryId:     e.Id,
			Name:        e.Name,
			Weight:      int32(e.Weight),
			Remaining:   int32(e.Remaining),
			IsPrize:     e.IsPrize == 1,
			Probability: fmt.Sprintf("%.2f%%", p),
		})
	}
	return &draw.BoxDetailResp{
		BoxId:       b.Id,
		BoxName:     b.BoxName,
		Price:       utils.FenToYuan(b.Price),
		Description: b.Description,
		SoldOut:     !prizeDrawable,
		Entries:     items,
	}, nil
}
