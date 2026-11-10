// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package pool

import (
	"context"

	"gacha_srv/app/draw/draw"
	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBoxLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// GetBox 读取箱子（gacha.pool:read；本仓箱子即 draw_boxes，映射 BoxDetail）。
func NewGetBoxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBoxLogic {
	return &GetBoxLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetBoxLogic) GetBox(req *draw.BoxDetailReq) (*draw.BoxDetailResp, error) {
	return l.svcCtx.DrawRpc.BoxDetail(l.ctx, req)
}
