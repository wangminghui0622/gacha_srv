package gameimpl

import (
	"context"
	"errors"
	"fmt"
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

	var outRecordId int64
	outAwards := make([]core.ComboChainAward, 0)
	outSettled := false
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleComboChain", func(db *gorm.DB) error {

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

		state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链状态数据异常", errorx.CodeGameConfigInvalid))
		}

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

		if st.RemainingTickets == 0 {

			if int64(len(state.Participants)) != cfg.TicketsPerCar {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链参与人数与配置不符", errorx.CodeComboChainConfigChanged))
			}

			carGlobals := core.AssignCarGlobals(slots, int(st.CarIndex), int(cfg.TotalCars))
			if len(carGlobals) == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链当前车无全局奖", errorx.CodeComboChainConfigChanged))
			}

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

			for i := range awards {
				awards[i].RequestID = in.RequestId
				awards[i].CarIndex = int32(st.CarIndex)
			}
			outAwards = awards

			snapshot := make(map[int64]core.ComboChainStreak, len(state.Participants))
			for _, pid := range state.Participants {
				snapshot[pid] = streaks[pid]
			}

			for userId, s := range streakNext {
				if err := upsertComboChainStreak(db, in.ConfigId, userId, s); err != nil {
					return err
				}
			}

			updates := map[string]interface{}{
				"awards_json":         comboChainAwardsJSON(append(state.Awards, awards...)),
				"tier_remaining_json": comboChainTierRemainingJSON(tierNext),
				"streaks_before_json": comboChainStreaksBeforeJSON(snapshot),
				"updated_at":          time.Now(),
			}
			if st.CarIndex >= cfg.TotalCars {

				outSettled = true
				updates["status"] = comboChainStateSettled
				updates["remaining_tickets"] = 0
			} else {

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

func NewSettleComboChainRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleComboChainRevertLogic {
	return &SettleComboChainRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleComboChainRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar/gashapon 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② 若本 gid 触发过整车结算（awards_json 有本 requestId 的结算奖）：
//	   按 requestId 删结算奖；COMBO_TIER 档位库存回补（combo_index++）；
//	   连击计数按 streaks_before_json 快照回滚；车次回退——car_index=记录车号、
//	   参与者由该车 status=1 记录重建（本 gid 记录已置 2 故自动排除）、
//	   余票=tickets_per_car-重建人数、状态 OPEN。
//
// 参与者弹出与余票回补由 03 补偿（ReleaseComboChain）唯一负责，本分支不碰余票
// 的普通回补（重建路径例外：整车结算回滚必须复原整辆车）。
func (l *SettleComboChainRevertLogic) SettleComboChainRevert(in *play.SettleComboChainReq) (*play.SettleComboChainResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleComboChainRevert", func(db *gorm.DB) error {

		// ① 记录 FOR UPDATE（盲撤销幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.ComboChainRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == comboChainRecordNormal {
				if err := db.Model(&gamemodelnc.ComboChainRecords{}).
					Where("id = ? AND status = ?", rec.Id, comboChainRecordNormal).
					Update("status", comboChainRecordCanceled).Error; err != nil {
					return err
				}

				if err := l.revertSettlement(db, &rec); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleComboChainRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleComboChainRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.ComboChainRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleComboChainRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleComboChainResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

// revertSettlement 整车结算回滚：删结算奖、档位回补、连击快照回滚、车次回退
func (l *SettleComboChainRevertLogic) revertSettlement(db *gorm.DB, rec *gamemodelnc.ComboChainRecords) error {
	var st gamemodelnc.ComboChainState
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("config_id = ?", rec.ConfigId).First(&st).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err != nil {
		return nil
	}
	state, perr := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
	if perr != nil {
		return nil
	}

	kept := make([]core.ComboChainAward, 0, len(state.Awards))
	removed := 0
	for _, a := range state.Awards {
		if a.RequestID != rec.RequestId {
			kept = append(kept, a)
			continue
		}
		removed++
		if a.Kind == core.ComboAwardKindCombo {
			if idx := int(a.ComboIndex); idx >= 0 && idx < len(state.TierRemaining) {
				state.TierRemaining[idx]++
			}
		}
	}
	if removed == 0 {

		return nil
	}

	for userId, s := range state.StreaksBefore {
		if err := upsertComboChainStreak(db, rec.ConfigId, userId, s); err != nil {
			return err
		}
	}
	// 车次回退：参与者由该车 status=1 记录重建（本 gid 已置 2 自动排除）
	var recs []gamemodelnc.ComboChainRecords
	if err := db.Where("config_id = ? AND car_index = ? AND status = ?", rec.ConfigId, rec.CarIndex, comboChainRecordNormal).
		Order("id ASC").Find(&recs).Error; err != nil {
		return err
	}
	participants := make([]int64, 0, len(recs))
	for _, r := range recs {
		participants = append(participants, r.UserId)
	}
	updates := map[string]interface{}{
		"awards_json":         comboChainAwardsJSON(kept),
		"tier_remaining_json": comboChainTierRemainingJSON(state.TierRemaining),
		"car_index":           rec.CarIndex,
		"participants_json":   comboChainParticipantsJSON(participants),
		"status":              comboChainStateOpen,
		"updated_at":          time.Now(),
	}
	// 余票复原 = tickets_per_car - 重建人数（配置缺失时保持原值防御）
	var cfg gamemodel.ComboChainConfigs
	if err := db.Where("id = ?", rec.ConfigId).First(&cfg).Error; err == nil {
		updates["remaining_tickets"] = cfg.TicketsPerCar - int64(len(participants))
	}
	if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
		Updates(updates).Error; err != nil {
		return err
	}
	l.Infof("revertSettlement: requestId: %s, removed: %d, participants rebuilt: %d",
		rec.RequestId, removed, len(participants))
	return nil
}
