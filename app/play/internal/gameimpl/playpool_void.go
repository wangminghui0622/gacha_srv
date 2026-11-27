package gameimpl

import (
	"context"
	"errors"
	"strings"

	"gacha_srv/app/play/internal/playpool"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var modeRuntimeConfigTable = map[string]string{
	"memorymatch": "memory_match_configs",
	"gashapon":    "gashapon_configs",
	"fullcar":     "full_car_configs",
	"combochain":  "combo_chain_configs",
	"dig":         "dig_box_configs",
	"groupbuy":    "group_buy_configs",
}

// VoidPlayPool 作废赏池：状态置 VOID，运行态立即下架；不可恢复。
// 已支付未开奖的退款/在途会话处理后续接入支付/订单域。
func VoidPlayPool(ctx context.Context, svcCtx *svc.ServiceContext, in *play.VoidPlayPoolReq) (*play.VoidPlayPoolResp, error) {
	if in.PoolId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "poolId 非法")
	}
	reason := strings.TrimSpace(in.Reason)
	err := svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cat playpool.CatalogRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", in.PoolId).First(&cat).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.InvalidArgument, "赏池不存在")
			}
			return err
		}
		if cat.Status == playpool.StatusVoid {
			return nil
		}
		if err := playpool.SetStatus(tx, in.PoolId, playpool.StatusVoid, reason, 0); err != nil {
			return err
		}
		if table := modeRuntimeConfigTable[cat.Mode]; table != "" {
			if err := tx.Table(table).Where("id = ?", in.PoolId).Update("status", 0).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &play.VoidPlayPoolResp{Status: "VOID"}, nil
}
