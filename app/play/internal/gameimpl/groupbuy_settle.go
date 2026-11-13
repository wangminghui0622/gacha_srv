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

func NewSettleGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGroupBuyLogic {
	return &SettleGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGroupBuy SAGA 05 正向：barrier 事务内
//
//	① state FOR UPDATE（必须 OPEN，种子缺失 → 3020）；
//	② 插记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	③ 余位==0（最后一注成团）→ 整批结算：
//	   普通槽=普通条目按 remaining 展开（不足 target → 3020）、条目子奖、
//	   幸运奖（lucky_sent 长度守卫 3034，remaining=quantity-sent）→
//	   core.SettleGroupBuyBatch（seed=gid 确定性）→ 结算奖标 requestId 追加、
//	   条目/子奖库存按结算奖计数扣减、lucky_sent 推进、状态 SETTLED。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleGroupBuyLogic) SettleGroupBuy(in *play.SettleGroupBuyReq) (*play.SettleGroupBuyResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 || in.Count <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 幂等重放快路径：同 requestId 记录已存在 → 读回成功（先于业务校验，M11b 教训）
	var existing gamemodelnc.GroupBuyRecords
	if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&existing).Error; err == nil {
		return l.readBackRecord(in.RequestId, in.UserId)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("SettleGroupBuy fast path query failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outAwards := make([]core.GroupBuyAward, 0)
	outSettled := false
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleGroupBuy", func(db *gorm.DB) error {

		// ① state FOR UPDATE（uk config_id 串行化；种子缺失 → 3020）
		var st gamemodelnc.GroupBuyState
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团状态不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if st.Status != groupBuyStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团已结算或状态不满足结算", errorx.CodeGroupBuyNoSlots))
		}

		// 拼团配置 + 幸运奖配置（配置 JSON 无效 → 3020）
		var cfg gamemodel.GroupBuyConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		luckyCfg, err := groupBuyParseLucky(cfg.LuckyPrizesJson)
		if err != nil || len(luckyCfg) == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团幸运奖配置无效", errorx.CodeGameConfigInvalid))
		}
		luckyChildren, err := groupBuyParseLuckyChildren(cfg.LuckyChildrenJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团幸运子奖配置无效", errorx.CodeGameConfigInvalid))
		}

		state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团状态数据异常", errorx.CodeGameConfigInvalid))
		}

		if len(state.LuckySent) != len(luckyCfg) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置变更，请重置", errorx.CodeGroupBuyConfigChanged))
		}

		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.GroupBuyRecords{
			RequestId: in.RequestId,
			UserId:    in.UserId,
			ConfigId:  in.ConfigId,
			Count:     in.Count,
			PriceFen:  priceFen,
			Status:    groupBuyRecordNormal,
			CreatedAt: time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errGroupBuyRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		if st.RemainingBets == 0 {
			// 条目快照：普通槽展开（parent=0 且 remaining>0）、条目子奖（parent!=0 且 remaining>0）
			var entries []gamemodel.GroupBuyEntries
			if err := db.Where("config_id = ?", in.ConfigId).Find(&entries).Error; err != nil {
				return err
			}
			if len(entries) == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团条目配置缺失", errorx.CodeGameConfigInvalid))
			}
			slots := make([]core.GroupBuySlot, 0)
			children := make([]core.GroupBuyChildStock, 0)
			for _, e := range entries {
				if e.ParentEntryId == 0 {
					for i := int64(0); i < e.Remaining; i++ {
						slots = append(slots, core.GroupBuySlot{
							EntryID:   e.Id,
							EntryName: e.Name,
							IsPrize:   e.IsPrize == 1,
							Remaining: int32(e.Remaining),
							PrizeKind: e.PrizeKind,
							SkuId:     e.SkuId,
						})
					}
					continue
				}
				if e.Remaining > 0 && e.PrizeKind != 0 {
					children = append(children, core.GroupBuyChildStock{
						EntryID:    e.Id,
						ParentID:   e.ParentEntryId,
						Remaining:  int32(e.Remaining),
						PrizeShape: core.PrizeShape{PrizeKind: e.PrizeKind, CashFen: e.CashFen, PointsAmount: e.PointsAmount, CouponTemplateNo: e.CouponTemplateNo, SkuId: e.SkuId, ItemName: e.Name},
					})
				}
			}
			if len(slots) < int(cfg.TargetBetCount) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团普通槽不足", errorx.CodeGameConfigInvalid))
			}

			lucky := make([]core.GroupBuyLuckyStock, 0, len(luckyCfg))
			for _, stock := range luckyCfg {
				sent := int32(0)
				if int(stock.Index) < len(state.LuckySent) {
					sent = state.LuckySent[stock.Index]
				}
				stock.Remaining = stock.Remaining - sent
				if stock.Remaining <= 0 {
					continue
				}
				for _, child := range luckyChildren {
					if child.Index == stock.Index {
						stock.Children = append(stock.Children, child)
					}
				}
				lucky = append(lucky, stock)
			}
			awards, err := core.SettleGroupBuyBatch(state.Bets, int(cfg.TargetBetCount),
				int(cfg.PerBetAwardLimit), slots, children, lucky, []byte(in.RequestId))
			if err != nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团结算失败", errorx.CodeGroupBuyConfigChanged))
			}
			for i := range awards {
				awards[i].RequestID = in.RequestId
			}
			outAwards = awards

			entryDelta := map[int64]int64{}
			luckyDelta := map[int32]int32{}
			for _, a := range awards {
				switch a.Kind {
				case core.GroupBuyAwardKindNormal:
					entryDelta[a.EntryID]++
				case core.GroupBuyAwardKindChild:
					if a.EntryID > 0 {
						entryDelta[a.EntryID]++
					}
				case core.GroupBuyAwardKindLucky:
					luckyDelta[a.LuckyIndex]++
				}
			}
			for entryId, n := range entryDelta {
				if err := db.Model(&gamemodel.GroupBuyEntries{}).
					Where("id = ? AND remaining >= ?", entryId, n).
					Update("remaining", gorm.Expr("remaining - ?", n)).Error; err != nil {
					return err
				}
			}
			for index, n := range luckyDelta {
				if int(index) < len(state.LuckySent) {
					state.LuckySent[index] += n
				}
			}
			outSettled = true
			if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
				Updates(map[string]interface{}{
					"awards_json":     groupBuyAwardsJSON(append(state.Awards, awards...)),
					"lucky_sent_json": groupBuyLuckySentJSON(state.LuckySent),
					"status":          groupBuyStateSettled,
					"updated_at":      time.Now(),
				}).Error; err != nil {
				return err
			}
			l.Infof("SettleGroupBuy settled, requestId: %s, awards: %d", in.RequestId, len(awards))
		}
		return nil
	})
	if errors.Is(err, errGroupBuyRecordDup) {

		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGroupBuy aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGroupBuy failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {

		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleGroupBuy success, requestId: %s, recordId: %d, settled: %v, awards: %d",
		in.RequestId, outRecordId, outSettled, len(outAwards))
	return &play.SettleGroupBuyResp{
		RecordId: outRecordId,
		Awards:   l.groupBuyAwardItems(outAwards),
		Settled:  outSettled,
		Status:   "SETTLED",
	}, nil
}

func NewSettleGroupBuyRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGroupBuyRevertLogic {
	return &SettleGroupBuyRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGroupBuyRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar/gashapon 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② 若本 gid 触发过整批结算（awards_json 有本 requestId 的结算奖）：
//	   按 requestId 删结算奖；条目库存回补（normal 按 entry_id 计数、
//	   条目子奖按 entry_id 计数；幸运子奖无库存不回补）；lucky_sent 逐幸运奖回退；
//	   SETTLED→OPEN。
//
// 下注记录弹出与余位回补由 03 补偿（ReleaseGroupBuy）唯一负责，本分支不碰余位。
func (l *SettleGroupBuyRevertLogic) SettleGroupBuyRevert(in *play.SettleGroupBuyReq) (*play.SettleGroupBuyResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleGroupBuyRevert", func(db *gorm.DB) error {

		// ① 记录 FOR UPDATE（盲撤销幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.GroupBuyRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == groupBuyRecordNormal {
				if err := db.Model(&gamemodelnc.GroupBuyRecords{}).
					Where("id = ? AND status = ?", rec.Id, groupBuyRecordNormal).
					Update("status", groupBuyRecordCanceled).Error; err != nil {
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
			l.Errorf("SettleGroupBuyRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGroupBuyRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.GroupBuyRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleGroupBuyRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleGroupBuyResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

// revertSettlement 整批结算回滚：删结算奖、条目库存回补、lucky_sent 回退、SETTLED→OPEN
func (l *SettleGroupBuyRevertLogic) revertSettlement(db *gorm.DB, rec *gamemodelnc.GroupBuyRecords) error {
	var st gamemodelnc.GroupBuyState
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("config_id = ?", rec.ConfigId).First(&st).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err != nil {
		return nil
	}
	state, perr := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
	if perr != nil {
		return nil
	}

	kept := make([]core.GroupBuyAward, 0, len(state.Awards))
	removed := 0
	entryDelta := map[int64]int64{}
	luckyDelta := map[int32]int32{}
	for _, a := range state.Awards {
		if a.RequestID != rec.RequestId {
			kept = append(kept, a)
			continue
		}
		removed++
		switch a.Kind {
		case core.GroupBuyAwardKindNormal:
			entryDelta[a.EntryID]++
		case core.GroupBuyAwardKindChild:
			if a.EntryID > 0 {
				entryDelta[a.EntryID]++
			}
		case core.GroupBuyAwardKindLucky:
			luckyDelta[a.LuckyIndex]++
		}
	}
	if removed == 0 {

		return nil
	}

	for entryId, n := range entryDelta {
		if err := db.Model(&gamemodel.GroupBuyEntries{}).
			Where("id = ?", entryId).
			Update("remaining", gorm.Expr("remaining + ?", n)).Error; err != nil {
			return err
		}
	}

	for index, n := range luckyDelta {
		if int(index) >= 0 && int(index) < len(state.LuckySent) {
			state.LuckySent[index] -= n
			if state.LuckySent[index] < 0 {
				state.LuckySent[index] = 0
			}
		}
	}

	updates := map[string]interface{}{
		"awards_json":     groupBuyAwardsJSON(kept),
		"lucky_sent_json": groupBuyLuckySentJSON(state.LuckySent),
		"status":          groupBuyStateOpen,
		"updated_at":      time.Now(),
	}
	if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
		Updates(updates).Error; err != nil {
		return err
	}
	l.Infof("revertSettlement: requestId: %s, removed: %d", rec.RequestId, removed)
	return nil
}
