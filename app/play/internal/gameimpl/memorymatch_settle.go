package gameimpl

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// mmRetryPendingSettlementGrant 补发 PENDING 结算奖（幂等 grant_no="mmr:<sessionID>"；重放 CompleteRound/Skip 时调用）
func mmRetryPendingSettlementGrant(ctx context.Context, svcCtx *svc.ServiceContext, sess *gamemodelnc.MemoryMatchSessions) error {
	var stl gamemodelnc.MemoryMatchSettlements
	if err := svcCtx.DB.Where("session_id = ?", sess.Id).First(&stl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if stl.Status != mmSettlementPending {
		return nil
	}
	var reward mmReward
	if err := json.Unmarshal([]byte(stl.RewardJson), &reward); err != nil {
		return err
	}
	if !reward.HasPrize {

		return svcCtx.DB.Model(&gamemodelnc.MemoryMatchSettlements{}).
			Where("session_id = ? AND status = ?", sess.Id, mmSettlementPending).
			Updates(map[string]interface{}{"status": mmSettlementGranted, "updated_at": time.Now()}).Error
	}
	grantNo := fmt.Sprintf("mmr:%d", sess.Id)
	if err := mmGrantOne(ctx, svcCtx, sess.UserId, grantNo, reward.PrizeKind, reward.CashFen, reward.PointsAmount, reward.CouponTemplate, reward.SkuId, reward.EntryName, sess.ConfigId); err != nil {
		return err
	}
	return svcCtx.DB.Model(&gamemodelnc.MemoryMatchSettlements{}).
		Where("session_id = ? AND status = ?", sess.Id, mmSettlementPending).
		Updates(map[string]interface{}{"status": mmSettlementGranted, "updated_at": time.Now()}).Error
}

// mmSettlementView 结算行 → 响应视图
func mmSettlementView(stl *gamemodelnc.MemoryMatchSettlements) (*play.MemoryMatchSettlementView, error) {
	var reward mmReward
	if err := json.Unmarshal([]byte(stl.RewardJson), &reward); err != nil {
		return nil, err
	}
	view := &play.MemoryMatchSettlementView{
		TotalPairCount: reward.TotalPairCount,
		FinalHash:      stl.ResultHash,
		Status:         stl.Status,
	}
	if reward.HasPrize {
		view.Prize = mmPrizeItem(reward.PrizeKind, reward.CashFen, reward.PointsAmount, reward.CouponTemplate, reward.EntryName, reward.SkuId)
	} else {
		view.Prize = &play.PrizeItem{Kind: 0}
	}
	return view, nil
}

// mmFinalSettlement 终局结算（调用方须已 FOR UPDATE 会话行；completed==purchased）：
//
//	① 1..purchased 回合全 RESOLVED/SKIPPED 且逐回合 Verify；
//	② total=Σpair_count、final_hash=SHA256("memory-match/v1/final"‖r1..rN)；
//	③ entries 按 total 精确匹配：有奖档 FOR UPDATE 扣 remaining（0 → 3045 售罄整事务回滚）；
//	④ 插 settlement(status=1 待发奖) + 会话 COMPLETED + 擂主换擂（事件 uk session_id 幂等、
//	   现任 pending → PENDING 打款流水 grant_no=mm-champion:<本会话ID>，事务外幂等补发）。
//
// 发奖在事务外（跨库钱包），mmRetryPendingSettlementGrant/mmRetryChampionPayouts 兜底。
func mmFinalSettlement(db *gorm.DB, sess *gamemodelnc.MemoryMatchSessions, rules *mmRules) (*gamemodelnc.MemoryMatchSettlements, int64, error) {
	var rounds []gamemodelnc.MemoryMatchRounds
	if err := db.Where("session_id = ?", sess.Id).Order("round_no ASC").Find(&rounds).Error; err != nil {
		return nil, 0, err
	}
	if int64(len(rounds)) != rules.RoundsPerPurchase {
		return nil, 0, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合数不完整")
	}
	total := int64(0)
	hashes := make([][]byte, 0, len(rounds))
	roundPairs := make(map[int64]int, len(rounds))
	for i := range rounds {
		if err := mmVerifyRoundResult(&rounds[i]); err != nil {
			return nil, 0, err
		}
		hashBytes, err := hex.DecodeString(*rounds[i].ResultHash)
		if err != nil || len(hashBytes) != 32 {
			return nil, 0, errorx.NewCodeError(errorx.CodeMemoryMatchConflict, "回合结果哈希异常")
		}
		total += rounds[i].PairCount
		hashes = append(hashes, hashBytes)
		roundPairs[rounds[i].RoundNo] = int(rounds[i].PairCount)
	}
	finalHash := hex.EncodeToString(memorymatch.FinalHash(hashes))

	entries, err := mmEntryOfConfig(db, sess.ConfigId)
	if err != nil {
		return nil, 0, err
	}
	var matched *gamemodel.MemoryMatchEntries
	for i := range entries {
		if entries[i].TotalPairCount == total {
			matched = &entries[i]
			break
		}
	}
	reward := mmReward{TotalPairCount: total, RoundPairs: roundPairs}
	if matched != nil && matched.IsPrize == 1 && matched.PrizeKind > 0 {
		// 有奖档：FOR UPDATE 扣库存（-1 不限；0 → 3045 售罄）
		var entry gamemodel.MemoryMatchEntries
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", matched.Id).First(&entry).Error; err != nil {
			return nil, 0, err
		}
		if entry.Remaining == 0 {
			return nil, 0, errorx.NewCodeError(errorx.CodeMemoryMatchPrizeExhausted, "奖档售罄")
		}
		if entry.Remaining > 0 {
			if err := db.Model(&gamemodel.MemoryMatchEntries{}).
				Where("id = ? AND remaining > 0", entry.Id).
				Update("remaining", gorm.Expr("remaining - 1")).Error; err != nil {
				return nil, 0, err
			}
		}
		reward.HasPrize = true
		reward.PrizeKind = entry.PrizeKind
		reward.CashFen = entry.CashFen
		reward.PointsAmount = entry.PointsAmount
		reward.CouponTemplate = entry.CouponTemplateNo
		reward.SkuId = entry.SkuId
		reward.EntryName = entry.Name
		reward.EntryID = entry.Id
		reward.Remaining = entry.Remaining
	} else if matched != nil {
		reward.EntryName = matched.Name
		reward.EntryID = matched.Id
	}

	rewardJSON, _ := json.Marshal(reward)
	stl := &gamemodelnc.MemoryMatchSettlements{
		SessionId:  sess.Id,
		ResultHash: finalHash,
		RewardJson: string(rewardJSON),
		Status:     mmSettlementPending,
		Version:    1,
	}
	if err := db.Create(stl).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", sess.Id).
		Updates(map[string]interface{}{
			"final_result_hash": finalHash,
			"status":            mmSessionCompleted,
			"version":           gorm.Expr("version + 1"),
			"updated_at":        time.Now(),
		}).Error; err != nil {
		return nil, 0, err
	}

	if err := mmDisplaceChampion(db, sess, rules, total); err != nil {
		return nil, 0, err
	}
	return stl, total, nil
}

func NewSettleMemoryMatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleMemoryMatchLogic {
	return &SettleMemoryMatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleMemoryMatch SAGA 05 正向：barrier 事务内
//
//	① 会话 FOR UPDATE（uk request_id 幂等重放：非 PENDING → 读回当前状态成功返回）；
//	② PENDING → AWAITING_DESTINY：engine_seed crypto/rand 落库；
//	③ 擂主分成：现任擂主 pending += 购买价×bps/10000（无擂主/未启用 → 跳过；
//	   记账到会话 champion_credit_user_id/champion_share_minor 供 Revert 等额回扣）。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleMemoryMatchLogic) SettleMemoryMatch(in *play.SettleMemoryMatchReq) (*play.SettleMemoryMatchResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outStatus int64
	var outSessionID int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleMemoryMatch", func(db *gorm.DB) error {

		// ① 会话 FOR UPDATE（request_id 唯一键幂等）
		var sess gamemodelnc.MemoryMatchSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=会话不存在", errorx.CodeMemoryMatchConflict))
			}
			return err
		}
		if in.SessionId > 0 && sess.Id != in.SessionId || sess.UserId != in.UserId || sess.ConfigId != in.ConfigId {
			return status.Error(codes.Aborted, "code=400 msg=会话参数不一致")
		}
		outStatus = sess.Status
		outSessionID = sess.Id
		if sess.Status != mmSessionPending {

			l.Infof("SettleMemoryMatch idempotent: session %d already at status %d", sess.Id, sess.Status)
			return nil
		}

		rules, err := mmSessionRules(&sess)
		if err != nil {
			return mmAborted(err)
		}

		if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ? AND status = ?", sess.Id, mmSessionPending).
			Updates(map[string]interface{}{
				"engine_seed": mmEngineSeedHex(),
				"status":      mmSessionAwaitingDestiny,
				"version":     gorm.Expr("version + 1"),
				"updated_at":  time.Now(),
			}).Error; err != nil {
			return err
		}

		if rules.ChampionMinTotalPairs > 0 && rules.ChampionShareBps > 0 {
			share := memorymatch.ChampionCreditMinor(sess.PurchasePriceMinor, rules.ChampionShareBps)
			if share > 0 {
				var ch gamemodelnc.MemoryMatchChampion
				err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("config_id = ?", sess.ConfigId).First(&ch).Error
				if err == nil {
					if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
						Update("pending_income_minor", gorm.Expr("pending_income_minor + ?", share)).Error; err != nil {
						return err
					}
					if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", sess.Id).
						Updates(map[string]interface{}{
							"champion_credit_user_id": ch.UserId,
							"champion_share_minor":    share,
						}).Error; err != nil {
						return err
					}
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}

			}
		}
		outStatus = mmSessionAwaitingDestiny
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleMemoryMatch aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleMemoryMatch failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功）：读库如实返回
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outStatus = sess.Status
			outSessionID = sess.Id
		}
	}
	l.Infof("SettleMemoryMatch success, requestId: %s, sessionId: %d, status: %d", in.RequestId, outSessionID, outStatus)
	return &play.SettleMemoryMatchResp{
		SessionId: outSessionID,
		State:     play.MemoryMatchSessionState(outStatus),
	}, nil
}

func NewSettleMemoryMatchRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleMemoryMatchRevertLogic {
	return &SettleMemoryMatchRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleMemoryMatchRevert SAGA 05 补偿：会话 AWAITING_DESTINY → PENDING（清引擎种子、回扣擂主分成）。
// 已进入游玩（AWAITING_SECOND/PLAYING/COMPLETED/EXPIRED）→ 跳过不打扰（盲撤销幂等）。
//
// 擂主分成回扣竞态守卫：仅当现任擂主仍是结算时记账的 champion_credit_user_id 才扣回
// （max(0, pending-share) 兜底）；擂主已易主则接受泄漏，绝不从新擂主处偷扣。
func (l *SettleMemoryMatchRevertLogic) SettleMemoryMatchRevert(in *play.SettleMemoryMatchReq) (*play.SettleMemoryMatchResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outStatus int64
	var outSessionID int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleMemoryMatchRevert", func(db *gorm.DB) error {

		var sess gamemodelnc.MemoryMatchSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		outSessionID = sess.Id
		outStatus = sess.Status
		if sess.Status != mmSessionAwaitingDestiny {
			return nil
		}

		if sess.ChampionCreditUserId > 0 && sess.ChampionShareMinor > 0 {
			var ch gamemodelnc.MemoryMatchChampion
			if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", sess.ConfigId).First(&ch).Error; err == nil {
				if ch.UserId == sess.ChampionCreditUserId {
					if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
						Update("pending_income_minor",
							gorm.Expr("GREATEST(0, pending_income_minor - ?)", sess.ChampionShareMinor)).Error; err != nil {
						return err
					}
				}

			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).
			Where("id = ? AND status = ?", sess.Id, mmSessionAwaitingDestiny).
			Updates(map[string]interface{}{
				"engine_seed":             gorm.Expr("NULL"),
				"champion_credit_user_id": 0,
				"champion_share_minor":    0,
				"status":                  mmSessionPending,
				"version":                 gorm.Expr("version + 1"),
				"updated_at":              time.Now(),
			}).Error; err != nil {
			return err
		}
		outStatus = mmSessionPending
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleMemoryMatchRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleMemoryMatchRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/已补偿）：读库如实返回
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outStatus = sess.Status
			outSessionID = sess.Id
		}
	}
	l.Infof("SettleMemoryMatchRevert success, requestId: %s, sessionId: %d, status: %d", in.RequestId, outSessionID, outStatus)
	return &play.SettleMemoryMatchResp{
		SessionId: outSessionID,
		State:     play.MemoryMatchSessionState(outStatus),
	}, nil
}
