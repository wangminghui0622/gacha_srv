// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPopupsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPopupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPopupsLogic {
	return &ListPopupsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListPopupsLogic) ListPopups(in *marketing.ListPopupsRequest) (*marketing.ListPopupsReply, error) {
	return l.svcCtx.MarketingRpc.ListPopups(l.ctx, in)
}
