package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type OperateRefundOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOperateRefundOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OperateRefundOrderLogic {
	return &OperateRefundOrderLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// OperateRefundOrder 退款单操作：审核通过 / 被迫退款 / 人工处理 / 重试退款。
// 当前为后台记账态迁移；真实支付渠道退款接入后替换这里的状态推进。
func (l *OperateRefundOrderLogic) OperateRefundOrder(in *money.OperateRefundOrderReq) (*money.RefundOrder, error) {
	if in.GetId() <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "退款订单ID无效")
	}
	action := in.GetAction()
	if action != money.RefundOrderAction_REFUND_ORDER_ACTION_APPROVE &&
		action != money.RefundOrderAction_REFUND_ORDER_ACTION_FORCE_REFUND &&
		action != money.RefundOrderAction_REFUND_ORDER_ACTION_MANUAL &&
		action != money.RefundOrderAction_REFUND_ORDER_ACTION_RETRY {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "退款操作无效")
	}
	row, err := repo.GetRefundOrder(l.svcCtx.DB, in.GetId())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeNotFound, "退款订单不存在")
		}
		return nil, errorx.FromMySQLError(err)
	}

	operator := strings.TrimSpace(in.GetAuditor())
	if operator == "" {
		operator = "管理员"
	}
	now := time.Now()
	updates := map[string]any{
		"auditor":         operator,
		"finance_auditor": operator,
	}
	if remark := strings.TrimSpace(in.GetRemark()); remark != "" {
		updates["audit_remark"] = remark
	}
	switch action {
	case money.RefundOrderAction_REFUND_ORDER_ACTION_APPROVE:
		updates["status"] = money.RefundOrderStatus_REFUND_ORDER_STATUS_SUCCESS
		updates["completed_at"] = &now
		updates["error_message"] = ""
	case money.RefundOrderAction_REFUND_ORDER_ACTION_FORCE_REFUND:
		updates["status"] = money.RefundOrderStatus_REFUND_ORDER_STATUS_SUCCESS
		updates["completed_at"] = &now
		updates["error_message"] = ""
	case money.RefundOrderAction_REFUND_ORDER_ACTION_MANUAL:
		updates["status"] = money.RefundOrderStatus_REFUND_ORDER_STATUS_MANUAL
		updates["completed_at"] = &now
	case money.RefundOrderAction_REFUND_ORDER_ACTION_RETRY:
		updates["status"] = money.RefundOrderStatus_REFUND_ORDER_STATUS_SUCCESS
		updates["completed_at"] = &now
		updates["error_message"] = ""
	}
	if err := repo.UpdateRefundOrder(l.svcCtx.DB, row.Id, updates); err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	updated, err := repo.GetRefundOrder(l.svcCtx.DB, row.Id)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return refundOrderToProto(updated), nil
}
