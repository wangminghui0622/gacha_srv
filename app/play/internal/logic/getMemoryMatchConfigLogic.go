package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMemoryMatchConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchConfigLogic {
	return &GetMemoryMatchConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMemoryMatchConfig 配置视图：玩法规则 + 奖档 + 现任擂主 + 本人外观（选择/默认 + 已拥有正面预设）
func (l *GetMemoryMatchConfigLogic) GetMemoryMatchConfig(in *play.GetMemoryMatchConfigReq) (*play.GetMemoryMatchConfigResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "configId 非法")
	}
	cfg, rules, err := mmLoadConfig(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, err
	}
	entries, err := mmEntryOfConfig(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	item, err := mmConfigItem(cfg, rules, entries)
	if err != nil {
		return nil, err
	}
	champion, err := mmChampionView(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	appearance, err := mmAppearanceView(l.svcCtx.DB, cfg, in.UserId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	return &play.GetMemoryMatchConfigResp{
		Config:     item,
		Champion:   champion,
		Appearance: appearance,
	}, nil
}
