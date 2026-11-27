package gameimpl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/catalog/catalog"
	"gacha_srv/app/catalog/catalogClient"
	"gacha_srv/app/play/internal/playpool"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// validatePlayPoolSKUs 生效前校验实物奖：存在、ACTIVE、PHYSICAL。
// 管理后台按「商品」选择实物奖（prizes.skuId 存商品 ID），也兼容直接存 SKU ID，
// 因此先按 SKU 查找，未命中再按商品查找。校验放在事务外（跨服务 RPC 不占用玩法库事务）。
func validatePlayPoolSKUs(ctx context.Context, svcCtx *svc.ServiceContext, poolID int64) error {
	if svcCtx.CatalogRpc == nil {
		return nil
	}
	p, err := playpool.Get(svcCtx.DB, poolID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	for _, prize := range p.Prizes {
		if prize.PrizeType != 2 {
			continue
		}
		sku, err := svcCtx.CatalogRpc.GetSKU(ctx, &catalogClient.GetSKURequest{SkuId: uint64(prize.SkuID)})
		if err == nil {
			if sku.Status != catalog.ProductStatus_PRODUCT_STATUS_ACTIVE {
				return status.Error(codes.InvalidArgument, fmt.Sprintf("奖品 %s 的 SKU 未上架", prize.Name))
			}
			if sku.FulfillmentMode != catalog.FulfillmentMode_FULFILLMENT_MODE_PHYSICAL {
				return status.Error(codes.InvalidArgument, fmt.Sprintf("奖品 %s 的 SKU 不是实物履约", prize.Name))
			}
			continue
		}
		reply, productErr := svcCtx.CatalogRpc.ListProducts(ctx, &catalogClient.ListProductsRequest{
			AfterProductId: uint64(prize.SkuID) - 1,
			PageSize:       1,
		})
		if productErr != nil || len(reply.Products) == 0 || reply.Products[0].ProductId != uint64(prize.SkuID) {
			return status.Error(codes.InvalidArgument, fmt.Sprintf("奖品 %s 的商品 %d 不存在或不可用", prize.Name, prize.SkuID))
		}
		if reply.Products[0].Status != catalog.ProductStatus_PRODUCT_STATUS_ACTIVE {
			return status.Error(codes.InvalidArgument, fmt.Sprintf("奖品 %s 的商品未上架", prize.Name))
		}
	}
	return nil
}

// PublishPlayPool 生效当前配置：投影到各玩法运行表，状态置 LIVE。
// 没有版本；重复调用幂等。旧参数 version 保留但忽略。
func PublishPlayPool(ctx context.Context, svcCtx *svc.ServiceContext, in *play.PublishPlayPoolReq) (*play.PublishPlayPoolResp, error) {
	if in.PoolId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "poolId 非法")
	}
	if err := validatePlayPoolSKUs(ctx, svcCtx, in.PoolId); err != nil {
		return nil, err
	}
	err := svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var cat playpool.CatalogRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", in.PoolId).First(&cat).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.InvalidArgument, "赏池不存在")
			}
			return err
		}
		switch cat.Status {
		case playpool.StatusLive:
			return nil
		case playpool.StatusVoid:
			return status.Error(codes.InvalidArgument, "赏池已作废，不能上线")
		case playpool.StatusDraft, playpool.StatusOffline:
		default:
			return status.Error(codes.InvalidArgument, "赏池状态不可上线")
		}
		p, err := playpool.Get(tx, in.PoolId)
		if err != nil {
			return err
		}
		if len(p.Prizes) == 0 {
			return status.Error(codes.InvalidArgument, "赏池没有奖品")
		}
		v := &gamemodelnc.PlayPoolVersion{
			PoolId:      p.ID,
			Version:     1,
			Status:      2,
			Mode:        p.Mode,
			Name:        p.Name,
			PriceFen:    p.PriceFen,
			ConfigJson:  p.ConfigJSON,
			ConfigHash:  "",
			PublishedAt: nil,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := materializePlayPool(tx, v, playPoolPrizesToLegacy(p)); err != nil {
			return err
		}
		if err := tx.Model(&playpool.CatalogRow{}).Where("id = ?", p.ID).Updates(map[string]any{
			"status":       playpool.StatusLive,
			"published_at": time.Now(),
			"updated_at":   time.Now(),
		}).Error; err != nil {
			return err
		}
		return playpool.WriteHistory(tx, p.ID, p.Mode, "APPLY", "", "", "", p.OperatorID)
	})
	if err != nil {
		return nil, err
	}
	return &play.PublishPlayPoolResp{Status: "PUBLISHED"}, nil
}

func playPoolPrizesToLegacy(p *playpool.Pool) []gamemodelnc.PlayPoolPrize {
	out := make([]gamemodelnc.PlayPoolPrize, 0, len(p.Prizes))
	for _, prize := range p.Prizes {
		isPrize := int32(0)
		if prize.IsPrize {
			isPrize = 1
		}
		out = append(out, gamemodelnc.PlayPoolPrize{
			PoolId:           p.ID,
			Version:          1,
			PrizeIndex:       int32(prize.Index),
			PrizeGrade:       prize.PrizeGrade,
			SpecialKind:      prize.SpecialKind,
			PrizeType:        int32(prize.PrizeType),
			Name:             prize.Name,
			SkuId:            prize.SkuID,
			ChildPoolId:      prize.ChildPoolID,
			Weight:           prize.Weight,
			Quantity:         int32(prize.Quantity),
			Remaining:        int32(prize.Quantity),
			IsPrize:          isPrize,
			TargetValue:      int32(prize.TargetValue),
			CashFen:          prize.CashFen,
			PointsAmount:     prize.PointsAmount,
			CouponTemplateNo: prize.CouponTemplateNo,
			CostFen:          prize.CostFen,
			ImageUrl:         prize.ImageURL,
			ParentIndex:      int32(prize.ParentIndex),
			ExtraJson:        prize.ExtraJSON,
			CreatedAt:        time.Now(),
		})
	}
	return out
}
