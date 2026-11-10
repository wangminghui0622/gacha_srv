package dig

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DigQuoteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDigQuoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DigQuoteLogic {
	return &DigQuoteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DigQuote 挖宝报价预览（UX 用）：格子数 × 单价 = gross。
// 价格在购买时由 game.LockDigBox 锁内重算强校验（6004），本端点纯展示。
func (l *DigQuoteLogic) DigQuote(req *types.DigQuoteReq) (resp *types.DigQuoteResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	// 单价取服务端列表
	var unitPrice string
	var cellCount int64
	boxes, err := l.svcCtx.PlayRpc.ListDigBoxes(l.ctx, &playClient.ListDigBoxesReq{})
	if err != nil {
		l.Errorf("DigQuote ListDigBoxes failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	for _, it := range boxes.Items {
		if it.BoxId == req.BoxId {
			unitPrice = it.Price
			cellCount = it.CellCount
			break
		}
	}
	if unitPrice == "" {
		return nil, errorx.NewCodeError(errorx.CodeDigBoxNotFound, "挖宝箱不存在")
	}

	cells, err := resolveDigCells(l.ctx, l.svcCtx, userId, req.BoxId, parseCellsCSV(req.Cells), req.TakeAll)
	if err != nil {
		return nil, err
	}
	_ = cellCount
	unitFen, err := utils.YuanToFen(unitPrice)
	if err != nil || unitFen <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeAmountInvalid, "挖宝价格无效")
	}
	grossFen := int64(len(cells)) * unitFen
	return &types.DigQuoteResp{
		BoxId:     req.BoxId,
		CellCount: cellCount,
		Count:     int64(len(cells)),
		UnitPrice: unitPrice,
		Gross:     utils.FenToYuan(grossFen),
		Payable:   utils.FenToYuan(grossFen),
	}, nil
}
