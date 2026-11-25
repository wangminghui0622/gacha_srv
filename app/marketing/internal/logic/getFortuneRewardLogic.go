package logic

import (
	"context"
	"errors"

	"gacha_srv/app/marketing/internal/repo"
	"gacha_srv/app/marketing/internal/svc"
	"gacha_srv/app/marketing/marketing"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetFortuneRewardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFortuneRewardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFortuneRewardLogic {
	return &GetFortuneRewardLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetFortuneReward 获取单条吉祥值奖励配置。
func (l *GetFortuneRewardLogic) GetFortuneReward(in *marketing.GetFortuneRewardRequest) (*marketing.FortuneReward, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "tenantId 须为 1（单租户）")
	}
	if in.Id == 0 {
		return nil, errorx.NewCodeError(errorx.CodeFortuneRewardInvalid, "id 不能为空")
	}
	row, err := repo.GetFortuneReward(l.svcCtx.DB, int64(in.Id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeFortuneRewardNotFound, "吉祥值奖励配置不存在")
		}
		return nil, err
	}
	return row.ToProto(), nil
}
