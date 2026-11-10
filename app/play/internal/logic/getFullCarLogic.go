package logic

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFullCarLogic {
	return &GetFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetFullCar 满车详情：配置 + 座位 + 余票 + 状态；整车奖仅在 DEPARTED 后可见。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetFullCarLogic) GetFullCar(in *play.GetFullCarReq) (*play.GetFullCarResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	cfg, err := l.svcCtx.FullCarCfgModel.FindOne(l.ctx, in.ConfigId)
	if err != nil {
		if errors.Is(err, gamemodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "满车配置不存在")
		}
		l.Errorf("GetFullCar query config failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	var st gamemodelnc.FullCarState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "满车状态不存在")
		}
		l.Errorf("GetFullCar query state failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	seats := fullCarParseSeats(st.SeatsJson)
	seatItems := make([]*play.FullCarSeatItem, 0, len(seats))
	for i, u := range seats {
		seatItems = append(seatItems, &play.FullCarSeatItem{SeatIndex: int64(i), UserId: u})
	}

	resp := &play.GetFullCarResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		SeatCount:        cfg.SeatCount,
		Seats:            seatItems,
		RemainingTickets: st.RemainingTickets,
		Status:           st.Status,
	}
	if st.Status == fullCarStateDeparted {
		resp.Awards = l.awardItems(fullCarParseAwards(st.AwardsJson))
	}
	return resp, nil
}

// awardItems 整车奖落库元素 → 响应条目（券名尽力解析，展示用）
func (l *GetFullCarLogic) awardItems(awards []fullCarStoredAward) []*play.FullCarAwardItem {
	items := make([]*play.FullCarAwardItem, 0, len(awards))
	for _, a := range awards {
		item := &play.PrizeItem{Kind: a.PrizeKind, Points: a.PointsAmount, CouponTemplateNo: a.CouponTemplateNo}
		switch a.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(a.CashFen)
		case prizeCoupon:
			item.CouponName = couponNameOf(l.ctx, l.svcCtx, a.CouponTemplateNo)
		}
		items = append(items, &play.FullCarAwardItem{
			SeatIndex: int64(a.SeatIndex),
			UserId:    a.UserId,
			Prize:     item,
			Tier:      int64(a.Tier),
		})
	}
	return items
}
