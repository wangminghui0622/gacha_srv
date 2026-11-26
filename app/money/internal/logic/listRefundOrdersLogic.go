package logic

import (
	"context"
	"time"

	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRefundOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRefundOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRefundOrdersLogic {
	return &ListRefundOrdersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListRefundOrdersLogic) ListRefundOrders(in *money.ListRefundOrdersReq) (*money.ListRefundOrdersResp, error) {
	page := int(in.GetPage())
	if page <= 0 {
		page = 1
	}
	filter := repo.RefundOrderFilter{
		Keyword:      in.GetKeyword(),
		AuditRemark:  in.GetAuditRemark(),
		Status:       in.GetStatus(),
		MinAmountFen: in.GetMinAmountFen(),
		MaxAmountFen: in.GetMaxAmountFen(),
		TimeField:    in.GetTimeField(),
		Page:         page,
		PageSize:     consolePageSize(in.GetPageSize()),
	}
	if in.GetStartAtUnixMillis() > 0 {
		t := time.UnixMilli(in.GetStartAtUnixMillis())
		filter.StartAt = &t
	}
	if in.GetEndAtUnixMillis() > 0 {
		t := time.UnixMilli(in.GetEndAtUnixMillis())
		filter.EndAt = &t
	}
	rows, total, err := repo.ListRefundOrders(l.svcCtx.DB, filter)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	resp := &money.ListRefundOrdersResp{Total: total}
	for i := range rows {
		resp.Items = append(resp.Items, refundOrderToProto(&rows[i]))
	}
	return resp, nil
}
