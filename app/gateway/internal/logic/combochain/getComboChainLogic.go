package combochain

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetComboChainLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetComboChainLogic {
	return &GetComboChainLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetComboChain 连击链详情：当前车进度/全局奖槽/共享奖档位/我的连击计数（JWT 组，需登录）
func (l *GetComboChainLogic) GetComboChain(configId int64) (resp *types.ComboChainDetailResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetComboChain(l.ctx, &playClient.GetComboChainReq{ConfigId: configId, UserId: userId})
	if err != nil {
		l.Errorf("GetComboChain failed, configId: %d, err: %v", configId, err)
		return nil, errorx.FromError(err)
	}
	slots := make([]types.DigPrizeItem, 0, len(r.GlobalSlots))
	for _, s := range r.GlobalSlots {
		slots = append(slots, digPrizeItemOf(s))
	}
	shared := make([]types.ComboChainSharedPrizeItem, 0, len(r.SharedPrizes))
	for _, p := range r.SharedPrizes {
		shared = append(shared, types.ComboChainSharedPrizeItem{
			Kind:                 p.Kind,
			Quantity:             p.Quantity,
			Remaining:            p.Remaining,
			TierWeight:           p.TierWeight,
			ConcentrateThreshold: p.ConcentrateThreshold,
			StreakThreshold:      p.StreakThreshold,
			Prize:                digPrizeItemOf(p.Prize),
		})
	}
	return &types.ComboChainDetailResp{
		ConfigId:         r.ConfigId,
		Name:             r.Name,
		Price:            r.Price,
		TicketsPerCar:    r.TicketsPerCar,
		TotalCars:        r.TotalCars,
		CarIndex:         r.CarIndex,
		Participants:     r.Participants,
		RemainingTickets: r.RemainingTickets,
		Status:           r.Status,
		GlobalSlots:      slots,
		SharedPrizes:     shared,
		EmptyStreak:      r.EmptyStreak,
		WinStreak:        r.WinStreak,
	}, nil
}
