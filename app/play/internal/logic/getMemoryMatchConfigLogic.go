package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

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

// ---- M15b 记忆翻牌 MemoryMatch（消息/语义照抄 server_v2 gacha memory_match）----
func (l *GetMemoryMatchConfigLogic) GetMemoryMatchConfig(in *play.GetMemoryMatchConfigReq) (*play.GetMemoryMatchConfigResp, error) {
	return gameimpl.NewGetMemoryMatchConfigLogic(l.ctx, l.svcCtx).GetMemoryMatchConfig(in)
}
