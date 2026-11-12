package gameimpl

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
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

func NewAssignGashaponLuckyUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignGashaponLuckyUserLogic {
	return &AssignGashaponLuckyUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AssignGashaponLuckyUser 幸运指派（仅 LUCKY 特别奖可指派；userId=0 删除）。
// 无 HTTP 路由（网关无管理员鉴权，挂上等于人人可指派——D4），供未来 ops-rpc/管理网关直调。
//
// 偏离记录：server_v2 校验用户存在；本仓 identity.proto 无查询用户 RPC，只校验
// prize 为 LUCKY（3027）。指派用户未参与时核心判定天然忽略（gashaponUserTouchesRange
// / participants 守卫），不产生错误发奖。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *AssignGashaponLuckyUserLogic) AssignGashaponLuckyUser(in *play.AssignGashaponLuckyUserReq) (*play.AssignGashaponLuckyUserResp, error) {
	if in.ConfigId <= 0 || in.PrizeIndex < 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	cfg, err := l.svcCtx.GashaponCfgModel.FindOne(l.ctx, in.ConfigId)
	if err != nil {
		if errors.Is(err, gamemodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋配置不存在")
		}
		l.Errorf("AssignGashaponLuckyUser query config failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}
	prizes, err := gashaponParsePrizes(cfg.SpecialPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋特别奖配置无效")
	}
	var target *core.GashaponSpecialPrize
	for i := range prizes {
		if int64(prizes[i].PrizeIndex) == in.PrizeIndex {
			target = &prizes[i]
			break
		}
	}
	if target == nil || target.SpecialKind != core.GashaponKindLucky {
		return nil, errorx.NewCodeError(errorx.CodeLuckyAssignInvalid, "幸运指派仅限 LUCKY 特别奖")
	}

	if in.UserId == 0 {
		if err := l.svcCtx.DB.Where("config_id = ? AND prize_index = ?", in.ConfigId, in.PrizeIndex).
			Delete(&gamemodelnc.GashaponLuckyAssignments{}).Error; err != nil {
			l.Errorf("AssignGashaponLuckyUser delete failed, configId: %d, prizeIndex: %d, err: %v", in.ConfigId, in.PrizeIndex, err)
			return nil, errorx.FromMySQLError(err)
		}
		l.Infof("AssignGashaponLuckyUser cleared, configId: %d, prizeIndex: %d", in.ConfigId, in.PrizeIndex)
		return &play.AssignGashaponLuckyUserResp{Status: "CLEARED"}, nil
	}

	// upsert（uk(config_id, prize_index)）：存在更新 user_id，缺失插入
	var existing gamemodelnc.GashaponLuckyAssignments
	err = l.svcCtx.DB.Where("config_id = ? AND prize_index = ?", in.ConfigId, in.PrizeIndex).
		First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("AssignGashaponLuckyUser query failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}
	if err == nil {
		if err := l.svcCtx.DB.Model(&gamemodelnc.GashaponLuckyAssignments{}).
			Where("id = ?", existing.Id).
			Updates(map[string]interface{}{"user_id": in.UserId, "updated_at": time.Now()}).Error; err != nil {
			l.Errorf("AssignGashaponLuckyUser update failed, err: %v", err)
			return nil, errorx.FromMySQLError(err)
		}
	} else {
		row := &gamemodelnc.GashaponLuckyAssignments{
			ConfigId:   in.ConfigId,
			PrizeIndex: in.PrizeIndex,
			UserId:     in.UserId,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := l.svcCtx.DB.Create(row).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {

				if err := l.svcCtx.DB.Model(&gamemodelnc.GashaponLuckyAssignments{}).
					Where("config_id = ? AND prize_index = ?", in.ConfigId, in.PrizeIndex).
					Updates(map[string]interface{}{"user_id": in.UserId, "updated_at": time.Now()}).Error; err != nil {
					l.Errorf("AssignGashaponLuckyUser upsert retry failed, err: %v", err)
					return nil, errorx.FromMySQLError(err)
				}
			} else {
				l.Errorf("AssignGashaponLuckyUser insert failed, err: %v", err)
				return nil, errorx.FromMySQLError(err)
			}
		}
	}
	l.Infof("AssignGashaponLuckyUser assigned, configId: %d, prizeIndex: %d, userId: %d", in.ConfigId, in.PrizeIndex, in.UserId)
	return &play.AssignGashaponLuckyUserResp{Status: "ASSIGNED"}, nil
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

func NewListGashaponRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGashaponRecordsLogic {
	return &ListGashaponRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListGashaponRecords 扭蛋记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListGashaponRecordsLogic) ListGashaponRecords(in *play.ListGashaponRecordsReq) (*play.ListGashaponRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	if in.RequestId != "" {
		var rec gamemodelnc.GashaponRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {

				return &play.ListGashaponRecordsResp{Items: []*play.GashaponRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListGashaponRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListGashaponRecordsResp{Items: []*play.GashaponRecordItem{l.buildItem(&rec)}, Total: 1}, nil
	}

	page, pageSize := in.Page, in.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := l.svcCtx.DB.Model(&gamemodelnc.GashaponRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListGashaponRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.GashaponRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListGashaponRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.GashaponRecordItem, 0, len(recs))
	for i := range recs {
		items = append(items, l.buildItem(&recs[i]))
	}
	return &play.ListGashaponRecordsResp{Items: items, Total: total}, nil
}

// loadAssignees 事务内读幸运指派：prize_index → 指派用户（0 值条目过滤）
func (l *SettleGashaponLogic) loadAssignees(db *gorm.DB, configId int64) map[int32]int64 {
	var assigns []gamemodelnc.GashaponLuckyAssignments
	if err := db.Where("config_id = ?", configId).Find(&assigns).Error; err != nil {
		l.Errorf("SettleGashapon load assignments failed, configId: %d, err: %v", configId, err)
		return map[int32]int64{}
	}
	out := make(map[int32]int64, len(assigns))
	for _, a := range assigns {
		out[int32(a.PrizeIndex)] = a.UserId
	}
	return out
}
