package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListDigBoxesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDigBoxesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigBoxesLogic {
	return &ListDigBoxesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 读 / 玩法 / 管理接口保留（下方 rpc 行原样保留）
func (l *ListDigBoxesLogic) ListDigBoxes(in *play.ListDigBoxesReq) (*play.ListDigBoxesResp, error) {
	return gameimpl.NewListDigBoxesLogic(l.ctx, l.svcCtx).ListDigBoxes(in)
}
