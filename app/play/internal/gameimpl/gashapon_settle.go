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

func NewSettleGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGashaponLogic {
	return &SettleGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGashapon SAGA 05 正向：barrier 事务内
//
//	① state FOR UPDATE（必须 OPEN）；sent 初始化/长度守卫（3026 配置变更）；
//	② 抽 count 个 regular 奖（gashapon_entries，每抽消耗一个条目实例，按条目余量
//	   递减——与 server_v2 persistent_draw 同款"每票耗一实例"语义）；
//	③ 插记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	④ 分阶段 LUCKY 判定（drawn=total-remaining_after_lock，seed=gid，含本次票据）；
//	⑤ 清空抽（remaining==0）→ 终局 LAST/MVP/LUCKY + status→SETTLED；
//	⑥ 新特别奖标 requestId 追加落库，sent 推进持久化。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleGashaponLogic) SettleGashapon(in *play.SettleGashaponReq) (*play.SettleGashaponResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 || in.Count <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 幂等重放快路径：同 requestId 记录已存在 → 读回成功（先于业务校验，M11b 教训）
	var existing gamemodelnc.GashaponRecords
	if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&existing).Error; err == nil {
		return l.readBackRecord(in.RequestId, in.UserId)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("SettleGashapon fast path query failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outOutcomes := make([]gashaponOutcome, 0, in.Count)
	outSpecial := make([]core.GashaponAward, 0)
	outSettled := false
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleGashapon", func(db *gorm.DB) error {

		// ① state FOR UPDATE（uk config_id 串行化；种子缺失 → 3020）
		var st gamemodelnc.GashaponState
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋状态不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if st.Status != gashaponStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋已结算或状态不满足结算", errorx.CodeGashaponNoTickets))
		}
		if st.RemainingTickets < 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋状态数据异常", errorx.CodeGameConfigInvalid))
		}

		// 扭蛋配置 + 特别奖（sent 长度与配置守卫：3026 配置变更）
		var cfg gamemodel.GashaponConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		prizes, err := gashaponParsePrizes(cfg.SpecialPrizesJson)
		if err != nil || len(prizes) == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋特别奖配置无效", errorx.CodeGameConfigInvalid))
		}

		state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋状态数据异常", errorx.CodeGameConfigInvalid))
		}
		if len(state.Sent) == 0 {
			state.Sent = make([]int32, len(prizes))
		}
		if len(state.Sent) != len(prizes) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋配置变更，请重置", errorx.CodeGashaponConfigChanged))
		}

		// ② 抽 count 个 regular 奖：总票数=Σ quantity（-1 非法）；奖池余量≥count
		var entries []gamemodel.GashaponEntries
		if err := db.Where("config_id = ?", in.ConfigId).Find(&entries).Error; err != nil {
			return err
		}
		var total int64
		remainingByName := map[int64]int64{}
		pool := make([]core.PrizeEntry, 0, len(entries))
		for _, e := range entries {
			if e.Quantity < 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋条目必须有限量", errorx.CodeGameConfigInvalid))
			}
			total += e.Quantity
			remainingByName[e.Id] = e.Remaining
			if e.Remaining > 0 {
				pool = append(pool, core.PrizeEntry{Id: e.Id, Name: e.Name, Weight: int(e.Weight)})
			}
		}
		if total <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋总票数非法", errorx.CodeGameConfigInvalid))
		}
		// ②.5 余量校验：全部条目 remaining 之和 ≥ count（有货条目数不含分摊余量，
		//      用条目数判断会在"少数条目余量充足"时误判余量不足）
		var totalRemaining int64
		for _, e := range pool {
			totalRemaining += remainingByName[e.Id]
		}
		if totalRemaining < in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋奖池余量不足", errorx.CodeGashaponNoTickets))
		}

		entryByName := map[int64]gamemodel.GashaponEntries{}
		for _, e := range entries {
			entryByName[e.Id] = e
		}
		for i := int64(0); i < in.Count; i++ {
			picked := core.PickPrizeDeterministic(pool, []byte(in.RequestId), "gashapon-regular", uint32(i))
			if picked == nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋奖池余量不足", errorx.CodeGashaponNoTickets))
			}
			if err := db.Model(&gamemodel.GashaponEntries{}).
				Where("id = ? AND remaining > 0", picked.Id).
				Update("remaining", gorm.Expr("remaining - 1")).Error; err != nil {
				return err
			}
			remainingByName[picked.Id]--
			outOutcomes = append(outOutcomes, gashaponOutcome{
				EntryId: picked.Id,
				Name:    picked.Name,
				IsPrize: entryByName[picked.Id].IsPrize == 1,
				SkuId:   entryByName[picked.Id].SkuId,
			})

			pool = filterGashaponPool(pool, picked.Id, remainingByName)
		}

		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.GashaponRecords{
			RequestId:   in.RequestId,
			UserId:      in.UserId,
			ConfigId:    in.ConfigId,
			Count:       in.Count,
			EntriesJson: gashaponOutcomesJSON(outOutcomes),
			PriceFen:    priceFen,
			Status:      gashaponRecordNormal,
			CreatedAt:   time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errGashaponRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		assignees := l.loadAssignees(db, in.ConfigId)
		drawn := total - st.RemainingTickets
		staged := core.StagedGashaponAwards(state, prizes, total, drawn, []byte(in.RequestId), assignees)

		// ⑤ 清空抽 → 终局 LAST/MVP/LUCKY 剩余 + SETTLED
		var final []core.GashaponAward
		if st.RemainingTickets == 0 {
			final = core.FinalGashaponAwards(state, prizes, in.UserId, []byte(in.RequestId), assignees)
			outSettled = true
		}

		newAwards := make([]core.GashaponAward, 0, len(staged)+len(final))
		for _, a := range append(append([]core.GashaponAward{}, staged...), final...) {
			a.RequestID = in.RequestId
			newAwards = append(newAwards, a)
		}
		outSpecial = newAwards
		updates := map[string]interface{}{
			"sent_json":   gashaponSentJSON(state.Sent),
			"awards_json": gashaponAwardsJSON(append(state.Awards, newAwards...)),
			"updated_at":  time.Now(),
		}
		if outSettled {
			updates["status"] = gashaponStateSettled
		}
		if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
			Updates(updates).Error; err != nil {
			return err
		}
		if outSettled {
			l.Infof("SettleGashapon settled, requestId: %s, special: %d", in.RequestId, len(newAwards))
		}
		return nil
	})
	if errors.Is(err, errGashaponRecordDup) {

		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGashapon aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGashapon failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {

		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleGashapon success, requestId: %s, recordId: %d, settled: %v, special: %d",
		in.RequestId, outRecordId, outSettled, len(outSpecial))
	return &play.SettleGashaponResp{
		RecordId:      outRecordId,
		RegularPrizes: gashaponOutcomesToGame(outOutcomes),
		SpecialAwards: l.specialAwardItems(outSpecial),
		Settled:       outSettled,
		Status:        "SETTLED",
	}, nil
}

func NewSettleGashaponRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGashaponRevertLogic {
	return &SettleGashaponRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGashaponRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② regular 奖库存回补（按记录 entries_json 逐条回补；同名条目多行时
//	   每单位回补一行 LIMIT 1——总库存按名复原，与抽中行号无关）；
//	③ awards_json 按 requestId 删本 gid 触发的特别奖、sent 逐奖回退；SETTLED→OPEN。
//
// 参与记录弹出与余票回补由 03 补偿（ReleaseGashapon）唯一负责，本分支不碰余票。
func (l *SettleGashaponRevertLogic) SettleGashaponRevert(in *play.SettleGashaponReq) (*play.SettleGashaponResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleGashaponRevert", func(db *gorm.DB) error {

		// ① 记录 FOR UPDATE → 2 已撤销（盲更新，幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.GashaponRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == gashaponRecordNormal {
				if err := db.Model(&gamemodelnc.GashaponRecords{}).
					Where("id = ? AND status = ?", rec.Id, gashaponRecordNormal).
					Update("status", gashaponRecordCanceled).Error; err != nil {
					return err
				}
			}

			for _, o := range gashaponParseOutcomes(rec.EntriesJson) {
				if o.EntryId > 0 {
					if err := db.Model(&gamemodel.GashaponEntries{}).
						Where("id = ?", o.EntryId).
						Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
						return err
					}
					continue
				}
				if err := db.Model(&gamemodel.GashaponEntries{}).
					Where("config_id = ? AND name = ?", rec.ConfigId, o.Name).
					Order("id ASC").Limit(1).
					Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
					return err
				}
			}
		}

		// ③ awards 按 requestId 删除 + sent 回退 + SETTLED→OPEN
		var st gamemodelnc.GashaponState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			state, perr := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
			updates := map[string]interface{}{"updated_at": time.Now()}
			if perr == nil {
				kept := make([]core.GashaponAward, 0, len(state.Awards))
				removed := 0
				for _, a := range state.Awards {
					if a.RequestID == in.RequestId {
						removed++

						if idx := int(a.PrizeIndex); idx >= 0 && idx < len(state.Sent) && state.Sent[idx] > 0 {
							state.Sent[idx]--
						}
						continue
					}
					kept = append(kept, a)
				}
				if removed > 0 {
					updates["awards_json"] = gashaponAwardsJSON(kept)
					updates["sent_json"] = gashaponSentJSON(state.Sent)
					l.Infof("SettleGashaponRevert awards removed, requestId: %s, removed: %d", in.RequestId, removed)
				}
			}
			if st.Status == gashaponStateSettled {
				updates["status"] = gashaponStateOpen
			}
			if len(updates) > 1 {
				if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
					Updates(updates).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGashaponRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGashaponRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.GashaponRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleGashaponRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleGashaponResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}
