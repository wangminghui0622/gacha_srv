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

type GetGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupBuyLogic {
	return &GetGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetGroupBuy 拼团详情视图：配置 + 余位 + 我的下注数 + 参与人数（去重）
// + 幸运奖（含已发进度）。状态行缺失时按种子语义回退（余位=target_bet_count/OPEN）。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetGroupBuyLogic) GetGroupBuy(in *play.GetGroupBuyReq) (*play.GetGroupBuyResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var cfg gamemodel.GroupBuyConfigs
	if err := l.svcCtx.DB.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "拼团配置不存在")
		}
		l.Errorf("GetGroupBuy query config failed, err: %v", err)
		return nil, err
	}

	// 幸运奖配置（含已发进度）
	luckyCfg, err := groupBuyParseLucky(cfg.LuckyPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "拼团幸运奖配置无效")
	}

	// 状态行（缺失回退种子语义：余位=target_bet_count、OPEN）
	remaining, status := cfg.TargetBetCount, groupBuyStateOpen
	myBets, participants := int64(0), int64(0)
	sent := make([]int32, len(luckyCfg))
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err == nil {
		remaining, status = st.RemainingBets, st.Status
		if state, perr := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson); perr == nil {
			if len(state.LuckySent) == len(luckyCfg) {
				sent = state.LuckySent
			}
			seen := map[int64]bool{}
			for _, b := range state.Bets {
				seen[b.UserID] = true
				if b.UserID == in.UserId {
					myBets += int64(b.Count)
				}
			}
			participants = int64(len(seen))
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("GetGroupBuy query state failed, err: %v", err)
		return nil, err
	}

	// 幸运奖视图（含进度）
	luckyItems := make([]*play.GroupBuyLuckyPrizeItem, 0, len(luckyCfg))
	for _, stock := range luckyCfg {
		sentCount := int64(0)
		if int(stock.Index) < len(sent) {
			sentCount = int64(sent[stock.Index])
		}
		luckyItems = append(luckyItems, &play.GroupBuyLuckyPrizeItem{
			PrizeIndex: int64(stock.Index),
			Quantity:   int64(stock.Remaining),
			Sent:       sentCount,
			Prize:      prizeShapeItem(l.ctx, l.svcCtx, stock.PrizeShape),
		})
	}

	return &play.GetGroupBuyResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		TargetBetCount:   cfg.TargetBetCount,
		PerBetAwardLimit: cfg.PerBetAwardLimit,
		RemainingBets:    remaining,
		MyBets:           myBets,
		Participants:     participants,
		Status:           status,
		LuckyPrizes:      luckyItems,
	}, nil
}
