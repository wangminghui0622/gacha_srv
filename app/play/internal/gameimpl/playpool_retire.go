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
)

// RetirePlayPool 下架赏池：把所有已发布版本置为 RETIRED（3），
// 对对碰同时把玩家端 memory_match_configs.status 置 0，主题列表不再展示。
// 草稿版本保留，后续可继续编辑并重新发布。
func RetirePlayPool(ctx context.Context, svcCtx *svc.ServiceContext, in *play.RetirePlayPoolReq) (*play.RetirePlayPoolResp, error) {
	if in.PoolId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "poolId 非法")
	}
	var retired int64
	err := svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var head gamemodelnc.PlayPoolHead
		if err := tx.Select("id, mode").Where("id = ?", in.PoolId).Take(&head).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.InvalidArgument, "赏池不存在")
			}
			return err
		}
		res := tx.Model(&gamemodelnc.PlayPoolVersion{}).
			Where("pool_id = ? AND status = ?", in.PoolId, 2).
			Updates(map[string]interface{}{
				"status":     3,
				"updated_at": time.Now(),
			})
		if res.Error != nil {
			return res.Error
		}
		retired = res.RowsAffected
		if head.Mode == "memorymatch" {
			if err := tx.Table("memory_match_configs").Where("id = ?", in.PoolId).
				Update("status", 0).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := "RETIRED"
	if retired == 0 {
		out = "NOT_PUBLISHED"
	}
	return &play.RetirePlayPoolResp{Status: out, RetiredVersions: retired}, nil
}
