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

// CreatePlayPoolDraft 创建统一玩赏池草稿（所有玩法共用）。
// 幂等键存在时：同 key 同请求返回原 pool/version；同 key 不同请求返回 Aborted。
func CreatePlayPoolDraft(ctx context.Context, svcCtx *svc.ServiceContext, in *play.CreatePlayPoolDraftReq) (*play.CreatePlayPoolDraftResp, error) {
	if err := validatePlayPoolDraft(in); err != nil {
		return nil, err
	}
	configJSON := in.ConfigJson
	if configJSON == "" {
		configJSON = "{}"
	}
	hash := hashPlayPoolDraft(in)

	var outPoolId, outVersion int64
	err := svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		// 幂等：同 key 已存在 → 校验请求哈希并返回原草稿
		if in.IdempotencyKey != "" {
			var d gamemodelnc.PlayPoolDraft
			err := tx.Where("idempotency_key = ?", in.IdempotencyKey).First(&d).Error
			if err == nil {
				if d.RequestHash != hash {
					return status.Error(codes.Aborted, "code=400 msg=幂等键冲突")
				}
				outPoolId, outVersion = d.PoolId, d.Version
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		var head gamemodelnc.PlayPoolHead
		if in.PoolId > 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ?", in.PoolId).First(&head).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return status.Error(codes.Aborted, "code=400 msg=赏池不存在")
				}
				return err
			}
			if head.Mode != in.Mode {
				return status.Error(codes.Aborted, "code=400 msg=赏池玩法不一致")
			}
		} else {
			head = gamemodelnc.PlayPoolHead{
				Mode:        in.Mode,
				Name:        in.Name,
				NextVersion: 1,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			if err := tx.Create(&head).Error; err != nil {
				return err
			}
		}

		version := head.NextVersion
		if err := tx.Model(&gamemodelnc.PlayPoolHead{}).Where("id = ?", head.Id).Updates(map[string]interface{}{
			"name":         in.Name,
			"next_version": version + 1,
			"updated_at":   time.Now(),
		}).Error; err != nil {
			return err
		}

		ver := &gamemodelnc.PlayPoolVersion{
			PoolId:     head.Id,
			Version:    version,
			Status:     1, // DRAFT
			Mode:       in.Mode,
			Name:       in.Name,
			PriceFen:   in.PriceFen,
			ConfigJson: configJSON,
			ConfigHash: hash,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := tx.Create(ver).Error; err != nil {
			return err
		}

		for _, p := range in.Prizes {
			row := modelPlayPoolPrize(p)
			row.PoolId = head.Id
			row.Version = version
			row.CreatedAt = time.Now()
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}

		if in.IdempotencyKey != "" {
			if err := tx.Create(&gamemodelnc.PlayPoolDraft{
				IdempotencyKey: in.IdempotencyKey,
				PoolId:         head.Id,
				Version:        version,
				RequestHash:    hash,
				CreatedAt:      time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		outPoolId, outVersion = head.Id, version
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &play.CreatePlayPoolDraftResp{
		PoolId:  outPoolId,
		Version: outVersion,
		Status:  "DRAFT",
	}, nil
}
