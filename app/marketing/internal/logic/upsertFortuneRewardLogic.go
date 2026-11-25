package logic

import (
	"context"
	"errors"
	"strings"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type UpsertFortuneRewardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertFortuneRewardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertFortuneRewardLogic {
	return &UpsertFortuneRewardLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// UpsertFortuneReward 创建/编辑吉祥值奖励配置（id=0 创建）。
func (l *UpsertFortuneRewardLogic) UpsertFortuneReward(in *marketing.UpsertFortuneRewardRequest) (*marketing.FortuneReward, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "tenantId 须为 1（单租户）")
	}
	r := in.Reward
	if r == nil {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "reward 不能为空")
	}
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || len([]rune(r.Name)) > 128 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "名称须为 1-128 个字符")
	}
	if r.ExchangePoints < 0 || r.DirectBuyMinor < 0 || r.CashbackMinor < 0 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "兑换积分/直购金额/返现金额不能为负数")
	}
	if r.Stock < 0 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "库存不能为负数")
	}
	if r.StartAtUnixMillis < 0 || r.EndAtUnixMillis < 0 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "开始/结束时间不能为负数")
	}
	if r.StartAtUnixMillis > 0 && r.EndAtUnixMillis > 0 && r.EndAtUnixMillis < r.StartAtUnixMillis {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "结束时间不能早于开始时间")
	}
	if len(r.Rewards) == 0 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "至少添加一个奖励")
	}
	for _, item := range r.Rewards {
		if item == nil || item.Type == marketing.FortuneRewardItemType_FORTUNE_REWARD_ITEM_TYPE_UNSPECIFIED {
			return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "奖励类型不能为空")
		}
	}
	row := repo.FortuneRewardFromProto(r)
	if r.Id == 0 {
		if err := repo.InsertFortuneReward(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	} else {
		existing, err := repo.GetFortuneReward(l.svcCtx.DB, int64(r.Id))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewCodeError(errorx.CodeFortuneRewardNotFound, "吉祥值奖励配置不存在")
			}
			return nil, err
		}
		row.ID = existing.ID
		if err := repo.UpdateFortuneReward(l.svcCtx.DB, row); err != nil {
			return nil, err
		}
	}
	saved, err := repo.GetFortuneReward(l.svcCtx.DB, row.ID)
	if err != nil {
		return nil, err
	}
	return saved.ToProto(), nil
}
