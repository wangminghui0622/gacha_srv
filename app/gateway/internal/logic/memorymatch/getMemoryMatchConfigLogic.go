package memorymatch

import (
	"context"

	playClient "gacha_srv/app/play/playClient"
	"gacha_srv/app/gateway/internal/middleware"
	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/gateway/internal/types"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMemoryMatchConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchConfigLogic {
	return &GetMemoryMatchConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetMemoryMatchConfig 玩法详情：配置+奖档+擂主+外观（GET /memory-match/configs/:configId）
func (l *GetMemoryMatchConfigLogic) GetMemoryMatchConfig(configId int64) (resp *types.MemoryMatchConfigResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetMemoryMatchConfig(l.ctx, &playClient.GetMemoryMatchConfigReq{
		ConfigId: configId,
		UserId:   userId,
	})
	if err != nil {
		l.Errorf("GetMemoryMatchConfig failed, configId: %d, err: %v", configId, err)
		return nil, errorx.FromError(err)
	}
	return &types.MemoryMatchConfigResp{
		Config:     configToView(r.Config),
		Champion:   championToView(r.Champion),
		Appearance: appearanceToView(r.Appearance),
	}, nil
}
