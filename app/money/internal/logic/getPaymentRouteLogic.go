package logic

import (
	"context"
	"errors"

	"gacha_srv/app/money/internal/core"
	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetPaymentRouteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPaymentRouteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPaymentRouteLogic {
	return &GetPaymentRouteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ── M14d 控制台：支付路由存管（半成品，无消费方；形状照抄 server_v2 payment_route）──
// D6/D7：tenant 固定 1；无行 → 2007。
func (l *GetPaymentRouteLogic) GetPaymentRoute(in *money.GetPaymentRouteRequest) (*money.GetPaymentRouteReply, error) {
	if err := core.ValidateGetPaymentRoute(in); err != nil {
		return nil, errorx.NewCodeError(errorx.CodePaymentRouteInvalid, err.Error())
	}
	row, err := repo.GetRoutePolicy(l.svcCtx.DB, tenantIDConsole, int32(in.Channel), int32(in.Scene))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodePaymentRouteNotFound, "支付路由策略不存在")
		}
		return nil, err
	}
	cands, err := repo.ListRouteCandidates(l.svcCtx.DB, row.ID)
	if err != nil {
		return nil, err
	}
	return routeReplyOf(row, cands), nil
}
