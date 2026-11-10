package gashapon

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGashaponLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGashaponLogic {
	return &GetGashaponLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetGashapon 扭蛋详情：特别奖进度/余票/参与人数/单价（JWT 组，需登录）
func (l *GetGashaponLogic) GetGashapon(configId int64) (resp *types.GashaponDetailResp, err error) {
	if _, ok := middleware.UserIdFromCtx(l.ctx); !ok {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetGashapon(l.ctx, &playClient.GetGashaponReq{ConfigId: configId})
	if err != nil {
		l.Errorf("GetGashapon failed, configId: %d, err: %v", configId, err)
		return nil, errorx.FromError(err)
	}
	prizes := make([]types.GashaponSpecialPrizeItem, 0, len(r.SpecialPrizes))
	for _, p := range r.SpecialPrizes {
		prizes = append(prizes, types.GashaponSpecialPrizeItem{
			PrizeIndex:  p.PrizeIndex,
			SpecialKind: p.SpecialKind,
			Quantity:    p.Quantity,
			Sent:        p.Sent,
			StageOrder:  p.StageOrder,
			Prize: types.DigPrizeItem{
				Kind:             p.Prize.Kind,
				Amount:           p.Prize.Amount,
				Points:           p.Prize.Points,
				CouponTemplateNo: p.Prize.CouponTemplateNo,
				CouponName:       p.Prize.CouponName,
			},
		})
	}
	return &types.GashaponDetailResp{
		ConfigId:         r.ConfigId,
		Name:             r.Name,
		Price:            r.Price,
		RemainingTickets: r.RemainingTickets,
		TotalTickets:     r.TotalTickets,
		Participants:     r.Participants,
		Status:           r.Status,
		SpecialPrizes:    prizes,
	}, nil
}
