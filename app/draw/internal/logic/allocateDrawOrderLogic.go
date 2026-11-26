package logic

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/draw/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AllocateDrawOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAllocateDrawOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AllocateDrawOrderLogic {
	return &AllocateDrawOrderLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *AllocateDrawOrderLogic) AllocateDrawOrder(in *draw.AllocateDrawOrderReq) (*draw.AllocateDrawOrderResp, error) {
	tx := l.svcCtx.DB.Begin()
	if tx.Error != nil {
		return nil, errorx.FromMySQLError(tx.Error)
	}
	defer func() { _ = tx.Rollback() }()

	alloc, err := allocateDrawOrder(tx, in.GetRequestId())
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "订单号分配失败："+err.Error())
	}
	if err := tx.Commit().Error; err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return &draw.AllocateDrawOrderResp{
		OrderId:  alloc.OrderID,
		OrderSeq: alloc.OrderSeq,
		OrderNo:  alloc.OrderNo,
	}, nil
}
