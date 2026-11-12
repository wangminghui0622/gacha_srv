package gameimpl

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

	luckyCfg, err := groupBuyParseLucky(cfg.LuckyPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "拼团幸运奖配置无效")
	}

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

func NewListGroupBuyRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGroupBuyRecordsLogic {
	return &ListGroupBuyRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListGroupBuyRecords 拼团记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 每条记录附本单触发的结算奖（state.awards_json 按 requestId 过滤，展示/轮询揭晓）。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListGroupBuyRecordsLogic) ListGroupBuyRecords(in *play.ListGroupBuyRecordsReq) (*play.ListGroupBuyRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	if in.RequestId != "" {
		var rec gamemodelnc.GroupBuyRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {

				return &play.ListGroupBuyRecordsResp{Items: []*play.GroupBuyRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListGroupBuyRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListGroupBuyRecordsResp{Items: []*play.GroupBuyRecordItem{l.buildItem(&rec)}, Total: 1}, nil
	}

	page, pageSize := in.Page, in.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := l.svcCtx.DB.Model(&gamemodelnc.GroupBuyRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListGroupBuyRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.GroupBuyRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListGroupBuyRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.GroupBuyRecordItem, 0, len(recs))
	for i := range recs {
		items = append(items, l.buildItem(&recs[i]))
	}
	return &play.ListGroupBuyRecordsResp{Items: items, Total: total}, nil
}
