package logic

import (
	"context"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchAppearanceLogic {
	return &GetMemoryMatchAppearanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetMemoryMatchAppearance 外观视图：当前选择（已存或默认）+ 已拥有正面预设
func (l *GetMemoryMatchAppearanceLogic) GetMemoryMatchAppearance(in *play.GetMemoryMatchAppearanceReq) (*play.GetMemoryMatchAppearanceResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "configId 非法")
	}
	cfg, _, err := mmLoadConfig(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, err
	}
	view, err := mmAppearanceView(l.svcCtx.DB, cfg, in.UserId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	return &play.GetMemoryMatchAppearanceResp{Appearance: view}, nil
}
