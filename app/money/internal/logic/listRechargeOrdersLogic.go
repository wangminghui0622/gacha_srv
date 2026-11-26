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

type ListRechargeOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRechargeOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRechargeOrdersLogic {
	return &ListRechargeOrdersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListRechargeOrdersLogic) ListRechargeOrders(in *money.ListRechargeOrdersReq) (*money.ListRechargeOrdersResp, error) {
	page := int(in.GetPage())
	if page <= 0 {
		page = 1
	}
	pageSize := consolePageSize(in.GetPageSize())
	filter := repo.RechargeOrderFilter{
		Keyword:      in.GetKeyword(),
		Status:       in.GetStatus(),
		RechargeType: in.GetRechargeType(),
		Platform:     in.GetPlatform(),
		PayKeyword:   in.GetPayKeyword(),
		MinAmountFen: in.GetMinAmountFen(),
		MaxAmountFen: in.GetMaxAmountFen(),
		TimeField:    in.GetTimeField(),
		Page:         page,
		PageSize:     pageSize,
	}
	if in.GetStartAtUnixMillis() > 0 {
		t := time.UnixMilli(in.GetStartAtUnixMillis())
		filter.StartAt = &t
	}
	if in.GetEndAtUnixMillis() > 0 {
		t := time.UnixMilli(in.GetEndAtUnixMillis())
		filter.EndAt = &t
	}
	rows, total, err := repo.ListRechargeOrders(l.svcCtx.DB, filter)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	resp := &money.ListRechargeOrdersResp{Total: total}
	for i := range rows {
		resp.Items = append(resp.Items, rechargeOrderToProto(&rows[i]))
	}
	return resp, nil
}
