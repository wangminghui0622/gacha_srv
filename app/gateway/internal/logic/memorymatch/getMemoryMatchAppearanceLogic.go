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

type GetMemoryMatchAppearanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchAppearanceLogic {
	return &GetMemoryMatchAppearanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetMemoryMatchAppearance 读外观：当前选择（已存或默认）+ 已拥有正面预设（GET /memory-match/appearance）
func (l *GetMemoryMatchAppearanceLogic) GetMemoryMatchAppearance(configId int64) (resp *types.MemoryMatchAppearanceResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.GetMemoryMatchAppearance(l.ctx, &playClient.GetMemoryMatchAppearanceReq{
		ConfigId: configId,
		UserId:   userId,
	})
	if err != nil {
		l.Errorf("GetMemoryMatchAppearance failed, configId: %d, err: %v", configId, err)
		return nil, errorx.FromError(err)
	}
	view := appearanceToView(r.Appearance)
	return &view, nil
}
