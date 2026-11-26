// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package money

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/money/money"

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
	return l.svcCtx.MoneyRpc.ListRechargeOrders(l.ctx, in)
}

type RefundRechargeOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefundRechargeOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundRechargeOrdersLogic {
	return &RefundRechargeOrdersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *RefundRechargeOrdersLogic) RefundRechargeOrders(in *money.RefundRechargeOrdersReq) (*money.RefundRechargeOrdersResp, error) {
	return l.svcCtx.MoneyRpc.RefundRechargeOrders(l.ctx, in)
}

type ListRefundOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRefundOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRefundOrdersLogic {
	return &ListRefundOrdersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListRefundOrdersLogic) ListRefundOrders(in *money.ListRefundOrdersReq) (*money.ListRefundOrdersResp, error) {
	return l.svcCtx.MoneyRpc.ListRefundOrders(l.ctx, in)
}

type OperateRefundOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOperateRefundOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OperateRefundOrderLogic {
	return &OperateRefundOrderLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *OperateRefundOrderLogic) OperateRefundOrder(in *money.OperateRefundOrderReq) (*money.RefundOrder, error) {
	return l.svcCtx.MoneyRpc.OperateRefundOrder(l.ctx, in)
}

type GetRefundAutoConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRefundAutoConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRefundAutoConfigLogic {
	return &GetRefundAutoConfigLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetRefundAutoConfigLogic) GetRefundAutoConfig(in *money.GetRefundAutoConfigReq) (*money.GetRefundAutoConfigResp, error) {
	return l.svcCtx.MoneyRpc.GetRefundAutoConfig(l.ctx, in)
}

type UpdateRefundAutoConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateRefundAutoConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRefundAutoConfigLogic {
	return &UpdateRefundAutoConfigLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateRefundAutoConfigLogic) UpdateRefundAutoConfig(in *money.UpdateRefundAutoConfigReq) (*money.UpdateRefundAutoConfigResp, error) {
	return l.svcCtx.MoneyRpc.UpdateRefundAutoConfig(l.ctx, in)
}
