package logic

import (
	"context"

	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRefundAutoConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRefundAutoConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRefundAutoConfigLogic {
	return &GetRefundAutoConfigLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetRefundAutoConfigLogic) GetRefundAutoConfig(_ *money.GetRefundAutoConfigReq) (*money.GetRefundAutoConfigResp, error) {
	cfg, err := repo.GetRefundAutoConfig(l.svcCtx.DB)
	if err != nil {
		return nil, errorx.FromMySQLError(err)
	}
	return &money.GetRefundAutoConfigResp{Config: refundAutoConfigToProto(cfg)}, nil
}
