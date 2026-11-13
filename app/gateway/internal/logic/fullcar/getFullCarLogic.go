package fullcar

import (
	"context"

	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFullCarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFullCarLogic {
	return &GetFullCarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetFullCar 满车详情：座位/整车奖（DEPARTED 后）/余票/单价（JWT 组，需登录）
func (l *GetFullCarLogic) GetFullCar(configId int64) (resp *types.FullCarDetailResp, err error) {
	if _, ok := middleware.UserIdFromCtx(l.ctx); !ok {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetFullCar(l.ctx, &playClient.GetFullCarReq{ConfigId: configId})
	if err != nil {
		l.Errorf("GetFullCar failed, configId: %d, err: %v", configId, err)
		return nil, errorx.FromError(err)
	}
	seats := make([]types.FullCarSeatItem, 0, len(r.Seats))
	for _, s := range r.Seats {
		seats = append(seats, types.FullCarSeatItem{SeatIndex: s.SeatIndex, UserId: s.UserId})
	}
	awards := make([]types.FullCarAwardItem, 0, len(r.Awards))
	for _, a := range r.Awards {
		awards = append(awards, types.FullCarAwardItem{
			SeatIndex: a.SeatIndex,
			UserId:    a.UserId,
			Prize: types.DigPrizeItem{
				Kind:             a.Prize.Kind,
				Amount:           a.Prize.Amount,
				Points:           a.Prize.Points,
				CouponTemplateNo: a.Prize.CouponTemplateNo,
				CouponName:       a.Prize.CouponName,
			},
			Tier: a.Tier,
		})
	}
	return &types.FullCarDetailResp{
		ConfigId:         r.ConfigId,
		Name:             r.Name,
		Price:            r.Price,
		SeatCount:        r.SeatCount,
		Seats:            seats,
		Awards:           awards,
		RemainingTickets: r.RemainingTickets,
		Status:           r.Status,
	}, nil
}
