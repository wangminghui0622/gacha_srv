package gameimpl

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/playpool"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// RetirePlayPool 下架赏池：状态置 OFFLINE，运行态停止展示；可再次生效。
func RetirePlayPool(ctx context.Context, svcCtx *svc.ServiceContext, in *play.RetirePlayPoolReq) (*play.RetirePlayPoolResp, error) {
	if in.PoolId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "poolId 非法")
	}
	var mode string
	err := svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cat playpool.CatalogRow
		if err := tx.Where("id = ?", in.PoolId).First(&cat).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.InvalidArgument, "赏池不存在")
			}
			return err
		}
		mode = cat.Mode
		if cat.Status == playpool.StatusVoid {
			return nil
		}
		if err := playpool.SetStatus(tx, in.PoolId, playpool.StatusOffline, "", 0); err != nil {
			return err
		}
		if mode == "memorymatch" {
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
	return &play.RetirePlayPoolResp{Status: "OFFLINE", RetiredVersions: 1}, nil
}
