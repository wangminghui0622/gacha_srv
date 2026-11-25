// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package promotion

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/marketing/marketing"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertPopupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertPopupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertPopupLogic {
	return &UpsertPopupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpsertPopupLogic) UpsertPopup(in *marketing.UpsertPopupRequest) (*marketing.Popup, error) {
	return l.svcCtx.MarketingRpc.UpsertPopup(l.ctx, in)
}
