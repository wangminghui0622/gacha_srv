package gameimpl

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PublishPlayPool 发布统一玩赏池版本。
//
// 在同一事务内按 mode 投影到各玩法现有运行表（gashapon/fullcar/combochain/
// dig/groupbuy/memorymatch），玩法运行态无需改动；重复发布幂等。
func PublishPlayPool(ctx context.Context, svcCtx *svc.ServiceContext, in *play.PublishPlayPoolReq) (*play.PublishPlayPoolResp, error) {
	if in.PoolId <= 0 || in.Version <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=poolId/version 非法")
	}
	out := "PUBLISHED"
	err := svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var v gamemodelnc.PlayPoolVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("pool_id = ? AND version = ?", in.PoolId, in.Version).First(&v).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, "code=400 msg=赏池版本不存在")
			}
			return err
		}
		if v.Status == 2 { // 幂等：已发布
			return nil
		}
		if v.Status != 1 {
			return status.Error(codes.Aborted, "code=400 msg=赏池版本状态不可发布")
		}

		var prizes []gamemodelnc.PlayPoolPrize
		if err := tx.Where("pool_id = ? AND version = ?", in.PoolId, in.Version).
			Order("prize_index ASC").Find(&prizes).Error; err != nil {
			return err
		}
		if len(prizes) == 0 {
			return status.Error(codes.Aborted, "code=400 msg=赏池没有奖品")
		}

		if err := materializePlayPool(tx, &v, prizes); err != nil {
			return err
		}

		if err := tx.Model(&gamemodelnc.PlayPoolVersion{}).
			Where("pool_id = ? AND version = ?", in.PoolId, in.Version).
			Updates(map[string]interface{}{
				"status":       2,
				"published_at": time.Now(),
				"updated_at":   time.Now(),
			}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &play.PublishPlayPoolResp{Status: out}, nil
}
