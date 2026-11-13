package gameimpl

import (
	"context"
	"errors"
	"fmt"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// mmRetryChampionPayouts 补发该配置 PENDING 擂主打款（grant_no 幂等；后续购买/结算时兜底）
func mmRetryChampionPayouts(ctx context.Context, svcCtx *svc.ServiceContext, configID int64) error {
	var rows []gamemodelnc.MemoryMatchChampionPayouts
	if err := svcCtx.DB.Where("config_id = ? AND status = ?", configID, mmPayoutPending).Find(&rows).Error; err != nil {
		return err
	}
	for _, p := range rows {
		if err := mmGrantOne(ctx, svcCtx, p.UserId, p.GrantNo, prizeCash, p.AmountMinor, 0, "", 0, "", 0); err != nil {
			return err
		}
		if err := svcCtx.DB.Model(&gamemodelnc.MemoryMatchChampionPayouts{}).
			Where("grant_no = ? AND status = ?", p.GrantNo, mmPayoutPending).
			Updates(map[string]interface{}{"status": mmPayoutPaid, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
	}
	return nil
}

// mmChampionView 现任擂主视图（无擂主 → exists=false）
func mmChampionView(db *gorm.DB, configID int64) (*play.MemoryMatchChampionInfo, error) {
	info := &play.MemoryMatchChampionInfo{}
	var ch gamemodelnc.MemoryMatchChampion
	if err := db.Where("config_id = ?", configID).First(&ch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return info, nil
		}
		return nil, err
	}
	info.Exists = true
	info.UserId = ch.UserId
	info.TotalPairCount = ch.TotalPairCount
	info.PendingIncome = utils.FenToYuan(ch.PendingIncomeMinor)
	return info, nil
}

// mmDisplaceChampion 擂主判定（结算事务内）：total≥min 且（无擂主或严格大于现任）→
// 插事件（uk session_id 防重）+ 现任 pending 全额 → PENDING 打款流水 + 更新现任；同用户只更新。
func mmDisplaceChampion(db *gorm.DB, sess *gamemodelnc.MemoryMatchSessions, rules *mmRules, total int64) error {
	if rules.ChampionMinTotalPairs <= 0 || total < rules.ChampionMinTotalPairs {
		return nil
	}
	var ch gamemodelnc.MemoryMatchChampion
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("config_id = ?", sess.ConfigId).First(&ch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {

		row := &gamemodelnc.MemoryMatchChampion{
			ConfigId: sess.ConfigId, UserId: sess.UserId,
			TotalPairCount: total, PendingIncomeMinor: 0, Version: 1,
		}
		if ierr := db.Create(row).Error; ierr != nil {
			if !errors.Is(ierr, gorm.ErrDuplicatedKey) {
				return ierr
			}
			if rerr := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", sess.ConfigId).First(&ch).Error; rerr != nil {
				return rerr
			}
		} else {
			ch = *row
		}
	} else if err != nil {
		return err
	}

	if ch.UserId == sess.UserId {

		if total > ch.TotalPairCount {
			if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
				Updates(map[string]interface{}{"total_pair_count": total, "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		}
		return nil
	}
	if total <= ch.TotalPairCount {
		return nil
	}

	if err := db.Create(&gamemodelnc.MemoryMatchChampionEvents{
		SessionId: sess.Id, UserId: sess.UserId, TotalPairCount: total,
	}).Error; err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return err
	}
	if ch.PendingIncomeMinor > 0 {
		if err := db.Create(&gamemodelnc.MemoryMatchChampionPayouts{
			ConfigId: sess.ConfigId, UserId: ch.UserId,
			AmountMinor: ch.PendingIncomeMinor,
			GrantNo:     fmt.Sprintf("mm-champion:%d", sess.Id),
			Status:      mmPayoutPending,
		}).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
		Updates(map[string]interface{}{
			"user_id": sess.UserId, "total_pair_count": total,
			"pending_income_minor": 0, "version": gorm.Expr("version + 1"),
		}).Error; err != nil {
		return err
	}
	return nil
}
