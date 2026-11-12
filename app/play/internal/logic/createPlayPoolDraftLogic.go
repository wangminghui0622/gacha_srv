package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlayPoolDraftLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePlayPoolDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlayPoolDraftLogic {
	return &CreatePlayPoolDraftLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- 统一赏池/奖品（所有玩法共用）----
func (l *CreatePlayPoolDraftLogic) CreatePlayPoolDraft(in *play.CreatePlayPoolDraftReq) (*play.CreatePlayPoolDraftResp, error) {
	return gameimpl.CreatePlayPoolDraft(l.ctx, l.svcCtx, in)
}
