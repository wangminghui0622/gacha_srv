// Code scaffolded by goctl. Safe to edit.
package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishPlayPoolLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublishPlayPoolLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishPlayPoolLogic {
	return &PublishPlayPoolLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// PublishPlayPool 发布统一玩赏池版本（gashapon 发布时投影到运行表）。
func (l *PublishPlayPoolLogic) PublishPlayPool(req *play.PublishPlayPoolReq) (*play.PublishPlayPoolResp, error) {
	return l.svcCtx.PlayRpc.PublishPlayPool(l.ctx, req)
}
