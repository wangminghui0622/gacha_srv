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

type GetGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGashaponLogic {
	return &GetGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetGashapon 扭蛋详情：配置 + 特别奖进度（sent/quantity）+ 余票/总票数/参与人数/状态。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetGashaponLogic) GetGashapon(in *play.GetGashaponReq) (*play.GetGashaponResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	cfg, err := l.svcCtx.GashaponCfgModel.FindOne(l.ctx, in.ConfigId)
	if err != nil {
		if errors.Is(err, gamemodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋配置不存在")
		}
		l.Errorf("GetGashapon query config failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	var st gamemodelnc.GashaponState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋状态不存在")
		}
		l.Errorf("GetGashapon query state failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	// 总票数 = Σ entries quantity（全部有限量）
	var total int64
	if err := l.svcCtx.DB.Model(&gamemodel.GashaponEntries{}).
		Where("config_id = ?", in.ConfigId).
		Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error; err != nil {
		l.Errorf("GetGashapon sum entries failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	prizes, err := gashaponParsePrizes(cfg.SpecialPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋特别奖配置无效")
	}
	state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋状态数据异常")
	}

	resp := &play.GetGashaponResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		RemainingTickets: st.RemainingTickets,
		TotalTickets:     total,
		Participants:     int64(len(state.Participants)),
		Status:           st.Status,
	}
	for _, p := range prizes {
		item := &play.PrizeItem{Kind: p.PrizeKind, Points: p.PointsAmount, CouponTemplateNo: p.CouponTemplateNo}
		switch p.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(p.CashFen)
		case prizeCoupon:
			item.CouponName = couponNameOf(l.ctx, l.svcCtx, p.CouponTemplateNo)
		}
		var sent int64
		if int(p.PrizeIndex) < len(state.Sent) {
			sent = int64(state.Sent[p.PrizeIndex])
		}
		resp.SpecialPrizes = append(resp.SpecialPrizes, &play.GashaponSpecialPrizeItem{
			PrizeIndex:  int64(p.PrizeIndex),
			SpecialKind: p.SpecialKind,
			Quantity:    int64(p.Quantity),
			Sent:        sent,
			StageOrder:  int64(p.StageOrder),
			Prize:       item,
		})
	}
	return resp, nil
}
