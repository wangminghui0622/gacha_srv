package dig

import (
	"context"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/playapi"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListDigRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDigRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigRecordsLogic {
	return &ListDigRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListDigRecords 挖宝记录（requestId 精确查询=购买轮询；空=分页列表）
func (l *ListDigRecordsLogic) ListDigRecords(req *types.DigRecordsReq) (resp *types.DigRecordsResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := playapi.ListDigRecords(l.ctx, l.svcCtx, req.RequestId, userId, req.Page, req.PageSize)
	if err != nil {
		l.Errorf("ListDigRecords failed, err: %v", err)
		return nil, errorx.FromError(err)
	}
	resp = &types.DigRecordsResp{Items: make([]types.DigRecordItem, 0, len(r.Items)), Total: r.Total}
	for _, it := range r.Items {
		prizes := make([]types.DigPrizeItem, 0, len(it.Prizes))
		for _, p := range it.Prizes {
			prizes = append(prizes, types.DigPrizeItem{
				Kind:             p.Kind,
				Amount:           p.Amount,
				Points:           p.Points,
				CouponTemplateNo: p.CouponTemplateNo,
				CouponName:       p.CouponName,
			})
		}
		resp.Items = append(resp.Items, types.DigRecordItem{
			RecordId:    it.RecordId,
			RequestId:   it.RequestId,
			ConfigId:    it.ConfigId,
			CellsChosen: it.CellsChosen,
			Prizes:      prizes,
			Price:       it.Price,
			Status:      it.Status,
			CreatedAt:   it.CreatedAt,
		})
	}
	return resp, nil
}
