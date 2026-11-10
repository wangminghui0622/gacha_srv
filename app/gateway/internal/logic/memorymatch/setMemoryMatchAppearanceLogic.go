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

type SetMemoryMatchAppearanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSetMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetMemoryMatchAppearanceLogic {
	return &SetMemoryMatchAppearanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SetMemoryMatchAppearance 存外观（PUT /memory-match/appearance；非默认 face 需权益 → 3044）
func (l *SetMemoryMatchAppearanceLogic) SetMemoryMatchAppearance(req *types.MemoryMatchAppearanceBody) (resp *types.MemoryMatchAppearanceResp, err error) {
	userId, ok := middleware.UserIdFromCtx(l.ctx)
	if !ok || userId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeUnauthorized, "未登录")
	}
	r, err := l.svcCtx.PlayRpc.SetMemoryMatchAppearance(l.ctx, &playClient.SetMemoryMatchAppearanceReq{
		ConfigId:     req.ConfigId,
		UserId:       userId,
		FacePresetId: req.FacePresetId,
		BackPresetId: req.BackPresetId,
	})
	if err != nil {
		l.Errorf("SetMemoryMatchAppearance failed, configId: %d, err: %v", req.ConfigId, err)
		return nil, errorx.FromError(err)
	}
	view := appearanceToView(r.Appearance)
	return &view, nil
}
