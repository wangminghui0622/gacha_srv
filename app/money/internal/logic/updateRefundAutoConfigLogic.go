package logic

import (
	"context"

	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateRefundAutoConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateRefundAutoConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRefundAutoConfigLogic {
	return &UpdateRefundAutoConfigLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateRefundAutoConfigLogic) UpdateRefundAutoConfig(in *money.UpdateRefundAutoConfigReq) (*money.UpdateRefundAutoConfigResp, error) {
	cfg := in.GetConfig()
	if cfg == nil {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "自动退款配置不能为空")
	}
	if cfg.GetRemainingAmountFen() < 0 || cfg.GetAuditLimitFen() < 0 ||
		cfg.GetDelaySeconds() < 0 || cfg.GetAlternateCount() < 0 || cfg.GetAlternateAmountFen() < 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "自动退款配置不能为负数")
	}
	row := &repo.RefundAutoConfig{
		Id:                 1,
		RemainingAmountFen: cfg.GetRemainingAmountFen(),
		AuditLimitFen:      cfg.GetAuditLimitFen(),
		DelaySeconds:       cfg.GetDelaySeconds(),
		AlternateCount:     cfg.GetAlternateCount(),
		AlternateAmountFen: cfg.GetAlternateAmountFen(),
	}
	if err := repo.SaveRefundAutoConfig(l.svcCtx.DB, row); err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return &money.UpdateRefundAutoConfigResp{Config: refundAutoConfigToProto(row)}, nil
}
