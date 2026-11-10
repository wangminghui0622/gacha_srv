package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// errComboChainRecordDup 事务内哨兵：同 requestId 的连击链记录已存在（另一 gid 已结算）
var errComboChainRecordDup = errors.New("combo chain record duplicated")

type SettleComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleComboChainLogic {
	return &SettleComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleComboChain SAGA 05 正向：barrier 事务内
//
//	① state FOR UPDATE（必须 OPEN，种子缺失 → 3020）；
//	② 插记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	③ 余票==0（最后一票）→ 整车结算：
//	   参与人数/全局奖槽守卫（3031）→ 档位库存构建 → 连击计数读入 →
//	   core.SettleComboCar（seed=gid 确定性）→ 结算奖标 requestId+车号追加、
//	   档位库存/连击推进/快照落库 → 车次推进（最后一车 → SETTLED 余票 0；
//	   否则车号+1、参与者清空、余票重置 tickets_per_car）。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleComboChainLogic) SettleComboChain(in *play.SettleComboChainReq) (*play.SettleComboChainResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 幂等重放快路径：同 requestId 记录已存在 → 读回成功（先于业务校验，M11b 教训）
	var existing gamemodelnc.ComboChainRecords
	if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&existing).Error; err == nil {
		return l.readBackRecord(in.RequestId, in.UserId)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("SettleComboChain fast path query failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleComboChain get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleComboChain get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outAwards := make([]core.ComboChainAward, 0)
	outSettled := false
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① state FOR UPDATE（uk config_id 串行化；种子缺失 → 3020）
		var st gamemodelnc.ComboChainState
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链状态不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if st.Status != comboChainStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链已结算或状态不满足结算", errorx.CodeComboChainNoTickets))
		}

		// 连击链配置 + 全局奖槽 + 共享奖（配置 JSON 无效 → 3020）
		var cfg gamemodel.ComboChainConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		slots, err := comboChainParseSlots(cfg.GlobalSlotsJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链全局奖槽配置无效", errorx.CodeGameConfigInvalid))
		}
		shared, err := comboChainParseShared(cfg.SharedPrizesJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链共享奖配置无效", errorx.CodeGameConfigInvalid))
		}

		// 状态 JSON → 内存
		state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链状态数据异常", errorx.CodeGameConfigInvalid))
		}

		// ② 插记录（uk request_id 幂等；撞键 = 已结算读回）
		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.ComboChainRecords{
			RequestId: in.RequestId,
			UserId:    in.UserId,
			ConfigId:  in.ConfigId,
			CarIndex:  st.CarIndex,
			PriceFen:  priceFen,
			Status:    comboChainRecordNormal,
			CreatedAt: time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errComboChainRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		// ③ 最后一票（余票==0）→ 整车结算
		if st.RemainingTickets == 0 {
			// 参与人数与配置守卫（3031 配置变更）：本车参与者必须满员
			if int64(len(state.Participants)) != cfg.TicketsPerCar {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链参与人数与配置不符", errorx.CodeComboChainConfigChanged))
			}
			// 本车全局奖槽（index%totalCars+1==carIndex）；缺失 → 3031
			carGlobals := core.AssignCarGlobals(slots, int(st.CarIndex), int(cfg.TotalCars))
			if len(carGlobals) == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链当前车无全局奖", errorx.CodeComboChainConfigChanged))
			}
			// 档位库存 + 连击计数
			tiers := core.BuildComboTierStock(shared, state.TierRemaining)
			streaks, err := loadComboChainStreaks(db, in.ConfigId)
			if err != nil {
				return err
			}
			awards, streakNext, tierNext, err := core.SettleComboCar(
				state.Participants, carGlobals, tiers, int32(cfg.TierTriggerProbabilityBps),
				shared, []byte(in.RequestId), streaks)
			if err != nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链结算失败", errorx.CodeComboChainConfigChanged))
			}
			// 结算奖标 requestId+车号追加
			for i := range awards {
				awards[i].RequestID = in.RequestId
				awards[i].CarIndex = int32(st.CarIndex)
			}
			outAwards = awards
			// 结算前连击快照（05 补偿回滚依据：只保留参与用户）
			snapshot := make(map[int64]core.ComboChainStreak, len(state.Participants))
			for _, pid := range state.Participants {
				snapshot[pid] = streaks[pid]
			}
			// 连击推进落库
			for userId, s := range streakNext {
				if err := upsertComboChainStreak(db, in.ConfigId, userId, s); err != nil {
					return err
				}
			}
			// 车次推进
			updates := map[string]interface{}{
				"awards_json":         comboChainAwardsJSON(append(state.Awards, awards...)),
				"tier_remaining_json": comboChainTierRemainingJSON(tierNext),
				"streaks_before_json": comboChainStreaksBeforeJSON(snapshot),
				"updated_at":          time.Now(),
			}
			if st.CarIndex >= cfg.TotalCars {
				// 最后一车：整链 SETTLED、余票 0（车号停在 total_cars）
				outSettled = true
				updates["status"] = comboChainStateSettled
				updates["remaining_tickets"] = 0
			} else {
				// 车次推进：参与者清空、余票重置
				updates["car_index"] = st.CarIndex + 1
				updates["participants_json"] = "[]"
				updates["remaining_tickets"] = cfg.TicketsPerCar
			}
			if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
				Updates(updates).Error; err != nil {
				return err
			}
			l.Infof("SettleComboChain settled, requestId: %s, car: %d, awards: %d",
				in.RequestId, st.CarIndex, len(awards))
		}
		return nil
	})
	if errors.Is(err, errComboChainRecordDup) {
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleComboChain aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleComboChain failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleComboChain success, requestId: %s, recordId: %d, settled: %v, awards: %d",
		in.RequestId, outRecordId, outSettled, len(outAwards))
	return &play.SettleComboChainResp{
		RecordId: outRecordId,
		Awards:   l.comboChainAwardItems(outAwards),
		Settled:  outSettled,
		Status:   "SETTLED",
	}, nil
}

// comboChainAwardItems 结算奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleComboChainLogic) comboChainAwardItems(awards []core.ComboChainAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		items = append(items, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 结算奖 + 当前状态如实返回
func (l *SettleComboChainLogic) readBackRecord(requestId string, userId int64) (*play.SettleComboChainResp, error) {
	var rec gamemodelnc.ComboChainRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "连击链记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var awards []core.ComboChainAward
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == comboChainStateSettled
		if state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson); err == nil {
			awards = comboChainAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, awards: %d", requestId, rec.Id, settled, len(awards))
	return &play.SettleComboChainResp{
		RecordId: rec.Id,
		Awards:   l.comboChainAwardItems(awards),
		Settled:  settled,
		Status:   "SETTLED",
	}, nil
}
