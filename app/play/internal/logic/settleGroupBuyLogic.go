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

// errGroupBuyRecordDup 事务内哨兵：同 requestId 的拼团记录已存在（另一 gid 已结算）
var errGroupBuyRecordDup = errors.New("group buy record duplicated")

type SettleGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

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

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleGroupBuy get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleGroupBuy get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outAwards := make([]core.GroupBuyAward, 0)
	outSettled := false
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

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

		// 状态 JSON → 内存
		state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团状态数据异常", errorx.CodeGameConfigInvalid))
		}
		// 3034 守卫：lucky_sent 长度与幸运奖配置数不一致 = 配置变更（空数组同样视为变更），Aborted
		if len(state.LuckySent) != len(luckyCfg) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置变更，请重置", errorx.CodeGroupBuyConfigChanged))
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

		// ③ 最后一注（余位==0）→ 整批结算
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
						})
					}
					continue
				}
				if e.Remaining > 0 && e.PrizeKind != 0 {
					children = append(children, core.GroupBuyChildStock{
						EntryID:    e.Id,
						ParentID:   e.ParentEntryId,
						Remaining:  int32(e.Remaining),
						PrizeShape: core.PrizeShape{PrizeKind: e.PrizeKind, CashFen: e.CashFen, PointsAmount: e.PointsAmount, CouponTemplateNo: e.CouponTemplateNo},
					})
				}
			}
			if len(slots) < int(cfg.TargetBetCount) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团普通槽不足", errorx.CodeGameConfigInvalid))
			}
			// 幸运奖库存 = quantity - lucky_sent（3034 守卫已过）；幸运子奖按 lucky_index 挂接
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
			// 库存推进按结算奖计数反推：normal/条目子奖扣 entries.remaining、lucky 推进 lucky_sent
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
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
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
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
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

// groupBuyAwardItems 结算奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleGroupBuyLogic) groupBuyAwardItems(awards []core.GroupBuyAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		items = append(items, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 结算奖 + 当前状态如实返回
func (l *SettleGroupBuyLogic) readBackRecord(requestId string, userId int64) (*play.SettleGroupBuyResp, error) {
	var rec gamemodelnc.GroupBuyRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "拼团记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var awards []core.GroupBuyAward
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == groupBuyStateSettled
		if state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson); err == nil {
			awards = groupBuyAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, awards: %d", requestId, rec.Id, settled, len(awards))
	return &play.SettleGroupBuyResp{
		RecordId: rec.Id,
		Awards:   l.groupBuyAwardItems(awards),
		Settled:  settled,
		Status:   "SETTLED",
	}, nil
}
