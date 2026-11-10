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

// errGashaponRecordDup 事务内哨兵：同 requestId 的扭蛋记录已存在（另一 gid 已结算）
var errGashaponRecordDup = errors.New("gashapon record duplicated")

type SettleGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

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

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleGashapon get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleGashapon get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outOutcomes := make([]gashaponOutcome, 0, in.Count)
	outSpecial := make([]core.GashaponAward, 0)
	outSettled := false
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

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

		// 状态 JSON → 内存（票据区间/参与数/已发计数/已发奖）
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

		// 每抽耗一个条目实例（remaining 递减，0 移出候选——每票一实例，与 server_v2 同款）
		// 确定性抽取：seed=gid，index=第几抽（同一 (entries, gid, count) 结果确定，可回放核对）
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
			})
			// 候选池同步：余量归零移出
			pool = filterGashaponPool(pool, picked.Id, remainingByName)
		}

		// ③ 插记录（uk request_id 幂等；撞键 = 已结算读回）
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

		// ④ 分阶段 LUCKY（drawn=已抽总票数，含本次——03 已扣余票；seed=gid 确定性）
		assignees := l.loadAssignees(db, in.ConfigId)
		drawn := total - st.RemainingTickets
		staged := core.StagedGashaponAwards(state, prizes, total, drawn, []byte(in.RequestId), assignees)

		// ⑤ 清空抽 → 终局 LAST/MVP/LUCKY 剩余 + SETTLED
		var final []core.GashaponAward
		if st.RemainingTickets == 0 {
			final = core.FinalGashaponAwards(state, prizes, in.UserId, []byte(in.RequestId), assignees)
			outSettled = true
		}

		// ⑥ 新特别奖标 requestId 追加落库 + sent 持久化
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
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
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
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
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

// filterGashaponPool 候选池同步：抽中的条目余量归零后移出（其余原样保留）
func filterGashaponPool(pool []core.PrizeEntry, pickedId int64, remainingByName map[int64]int64) []core.PrizeEntry {
	if remainingByName[pickedId] > 0 {
		return pool
	}
	out := make([]core.PrizeEntry, 0, len(pool))
	for _, p := range pool {
		if p.Id == pickedId {
			continue
		}
		out = append(out, p)
	}
	return out
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

// gashaponOutcomesToGame regular 奖结果 → 响应条目
func gashaponOutcomesToGame(outcomes []gashaponOutcome) []*play.GashaponEntryOutcome {
	items := make([]*play.GashaponEntryOutcome, 0, len(outcomes))
	for _, o := range outcomes {
		items = append(items, &play.GashaponEntryOutcome{Name: o.Name, IsPrize: o.IsPrize})
	}
	return items
}

// specialAwardItems 特别奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleGashaponLogic) specialAwardItems(awards []core.GashaponAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		item := &play.PrizeItem{Kind: a.PrizeKind, Points: a.PointsAmount, CouponTemplateNo: a.CouponTemplateNo}
		switch a.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(a.CashFen)
		case prizeCoupon:
			item.CouponName = couponNameOf(l.ctx, l.svcCtx, a.CouponTemplateNo)
		}
		items = append(items, item)
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 特别奖 + 当前状态如实返回
func (l *SettleGashaponLogic) readBackRecord(requestId string, userId int64) (*play.SettleGashaponResp, error) {
	var rec gamemodelnc.GashaponRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "扭蛋记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var special []core.GashaponAward
	var st gamemodelnc.GashaponState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == gashaponStateSettled
		if state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson); err == nil {
			special = gashaponAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, special: %d", requestId, rec.Id, settled, len(special))
	return &play.SettleGashaponResp{
		RecordId:      rec.Id,
		RegularPrizes: gashaponOutcomesToGame(gashaponParseOutcomes(rec.EntriesJson)),
		SpecialAwards: l.specialAwardItems(special),
		Settled:       settled,
		Status:        "SETTLED",
	}, nil
}
