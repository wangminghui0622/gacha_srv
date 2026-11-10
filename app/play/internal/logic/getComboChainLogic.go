package logic

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetComboChainLogic {
	return &GetComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetComboChain 连击链详情视图：配置 + 当前车进度 + 全局奖槽 + 共享奖（含档位剩余）
// + 我的连击计数。状态行缺失时按种子语义回退（车 1/余票 tickets_per_car/OPEN）。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetComboChainLogic) GetComboChain(in *play.GetComboChainReq) (*play.GetComboChainResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var cfg gamemodel.ComboChainConfigs
	if err := l.svcCtx.DB.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "连击链配置不存在")
		}
		l.Errorf("GetComboChain query config failed, err: %v", err)
		return nil, err
	}

	// 状态行（缺失回退种子语义：车 1、余票=tickets_per_car、OPEN）
	carIndex, remaining, status := int64(1), cfg.TicketsPerCar, comboChainStateOpen
	participants := int64(0)
	var tierRemaining []int32
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err == nil {
		carIndex, remaining, status = st.CarIndex, st.RemainingTickets, st.Status
		if state, perr := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson); perr == nil {
			participants = int64(len(state.Participants))
			tierRemaining = state.TierRemaining
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("GetComboChain query state failed, err: %v", err)
		return nil, err
	}

	// 全局奖槽（配置 JSON 无效 → 3020）
	slots, err := comboChainParseSlots(cfg.GlobalSlotsJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "连击链全局奖槽配置无效")
	}
	globalItems := make([]*play.PrizeItem, 0, len(slots))
	for _, slot := range slots {
		globalItems = append(globalItems, prizeShapeItem(l.ctx, l.svcCtx, slot.PrizeShape))
	}

	// 共享奖（含档位剩余；连击奖无限量剩余=-1）
	shared, err := comboChainParseShared(cfg.SharedPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "连击链共享奖配置无效")
	}
	sharedItems := make([]*play.ComboChainSharedPrizeItem, 0, len(shared))
	var comboIndex int32
	for _, p := range shared {
		item := &play.ComboChainSharedPrizeItem{
			Kind:                 p.Kind,
			Quantity:             int64(p.Quantity),
			Remaining:            -1,
			TierWeight:           int64(p.TierWeight),
			ConcentrateThreshold: int64(p.ConcentrateThreshold),
			StreakThreshold:      int64(p.StreakThreshold),
			Prize:                prizeShapeItem(l.ctx, l.svcCtx, p.PrizeShape),
		}
		if p.Kind == core.ComboSharedKindComboTier {
			if int(comboIndex) < len(tierRemaining) {
				item.Remaining = int64(tierRemaining[comboIndex])
			} else {
				item.Remaining = int64(p.Quantity)
			}
			comboIndex++
		}
		sharedItems = append(sharedItems, item)
	}

	// 我的连击计数
	var emptyStreak, winStreak int64
	if in.UserId > 0 {
		var row gamemodelnc.ComboChainStreaks
		if err := l.svcCtx.DB.Where("config_id = ? AND user_id = ?", in.ConfigId, in.UserId).
			First(&row).Error; err == nil {
			emptyStreak, winStreak = row.EmptyStreak, row.WinStreak
		}
	}

	return &play.GetComboChainResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		TicketsPerCar:    cfg.TicketsPerCar,
		TotalCars:        cfg.TotalCars,
		CarIndex:         carIndex,
		Participants:     participants,
		RemainingTickets: remaining,
		Status:           status,
		GlobalSlots:      globalItems,
		SharedPrizes:     sharedItems,
		EmptyStreak:      emptyStreak,
		WinStreak:        winStreak,
	}, nil
}
