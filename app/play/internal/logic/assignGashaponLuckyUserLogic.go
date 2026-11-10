package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type AssignGashaponLuckyUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

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

	// 校验 prize 存在且为 LUCKY（3027）
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

	// 删除指派（userId=0）
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
				// 并发 upsert 竞争：重读后更新
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
